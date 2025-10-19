package cli

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"

	"github.com/dewadaru/mtgo/antireplay"
	"github.com/dewadaru/mtgo/events"
	"github.com/dewadaru/mtgo/internal/config"
	"github.com/dewadaru/mtgo/internal/utils"
	"github.com/dewadaru/mtgo/ipblocklist"
	"github.com/dewadaru/mtgo/ipblocklist/files"
	"github.com/dewadaru/mtgo/logger"
	"github.com/dewadaru/mtgo/mtglib"
	"github.com/dewadaru/mtgo/network"
	"github.com/rs/zerolog"
	"github.com/yl2chen/cidranger"
)

func makeLogger(conf *config.Config) mtglib.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.TimestampFieldName = "timestamp"
	zerolog.LevelFieldName = "level"

	if conf.Debug.Get(false) {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}

	baseLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	return logger.NewZeroLogger(baseLogger)
}

func makeNetwork(conf *config.Config, version string) (mtglib.Network, error) {
	tcpTimeout := conf.Network.Timeout.TCP.Get(network.DefaultTimeout)
	httpTimeout := conf.Network.Timeout.HTTP.Get(network.DefaultHTTPTimeout)
	dohIP := conf.Network.DOHIP.Get(net.ParseIP(network.DefaultDOHHostname)).String()
	userAgent := "mtg/" + version

	baseDialer, err := network.NewDefaultDialer(tcpTimeout, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot build a default dialer: %w", err)
	}

	if len(conf.Network.Proxies) == 0 {
		return network.NewNetwork(baseDialer, userAgent, dohIP, httpTimeout)
	}

	proxyURLs := make([]*url.URL, 0, len(conf.Network.Proxies))

	for _, v := range conf.Network.Proxies {
		if value := v.Get(nil); value != nil {
			proxyURLs = append(proxyURLs, value)
		}
	}

	if len(proxyURLs) == 1 {
		socksDialer, err := network.NewSocks5Dialer(baseDialer, proxyURLs[0])
		if err != nil {
			return nil, fmt.Errorf("cannot build socks5 dialer: %w", err)
		}

		return network.NewNetwork(socksDialer, userAgent, dohIP, httpTimeout)
	}

	socksDialer, err := network.NewLoadBalancedSocks5Dialer(baseDialer, proxyURLs)
	if err != nil {
		return nil, fmt.Errorf("cannot build socks5 dialer: %w", err)
	}

	return network.NewNetwork(socksDialer, userAgent, dohIP, httpTimeout)
}

func makeAntiReplayCache(conf *config.Config) mtglib.AntiReplayCache {
	if !conf.Defense.AntiReplay.Enabled.Get(false) {
		return antireplay.NewNoop()
	}

	return antireplay.NewStableBloomFilter(
		conf.Defense.AntiReplay.MaxSize.Get(antireplay.DefaultStableBloomFilterMaxSize),
		conf.Defense.AntiReplay.ErrorRate.Get(antireplay.DefaultStableBloomFilterErrorRate),
	)
}

func makeIPBlocklist(
	conf config.ListConfig,
	logger mtglib.Logger,
	ntw mtglib.Network,
	updateCallback ipblocklist.FireholUpdateCallback,
) (mtglib.IPBlocklist, error) {
	if !conf.Enabled.Get(false) {
		return ipblocklist.NewNoop(), nil
	}

	remoteURLs := make([]string, 0, len(conf.URLs))
	localFiles := make([]string, 0, len(conf.URLs)/4)

	for _, v := range conf.URLs {
		if v.IsRemote() {
			remoteURLs = append(remoteURLs, v.String())
		} else {
			localFiles = append(localFiles, v.String())
		}
	}

	blocklist, err := ipblocklist.NewFirehol(
		logger.Named("ipblockist"),
		ntw,
		conf.DownloadConcurrency.Get(1),
		remoteURLs,
		localFiles,
		updateCallback,
	)
	if err != nil {
		return nil, fmt.Errorf("incorrect parameters for firehol: %w", err)
	}

	go blocklist.Run(conf.UpdateEach.Get(ipblocklist.DefaultFireholUpdateEach))

	return blocklist, nil
}

func makeIPAllowlist(
	conf config.ListConfig,
	logger mtglib.Logger,
	ntw mtglib.Network,
	updateCallback ipblocklist.FireholUpdateCallback,
) (mtglib.IPBlocklist, error) {
	var (
		allowlist mtglib.IPBlocklist
		err       error
	)

	if !conf.Enabled.Get(false) {
		allowlist, err = ipblocklist.NewFireholFromFiles(
			logger.Named("ipblocklist"),
			1,
			[]files.File{
				files.NewMem([]*net.IPNet{
					cidranger.AllIPv4,
					cidranger.AllIPv6,
				}),
			},
			updateCallback,
		)

		go allowlist.Run(conf.UpdateEach.Get(ipblocklist.DefaultFireholUpdateEach))
	} else {
		allowlist, err = makeIPBlocklist(
			conf,
			logger,
			ntw,
			updateCallback,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("cannot build allowlist: %w", err)
	}

	return allowlist, nil
}

func makeEventStream(_ *config.Config) (mtglib.EventStream, error) {
	return events.NewNoopStream(), nil
}

func runProxy(conf *config.Config, version string) error {
	logger := makeLogger(conf)
	logger.BindJSON("configuration", conf.String()).Debug("configuration")

	eventStream, err := makeEventStream(conf)
	if err != nil {
		return fmt.Errorf("cannot build event stream: %w", err)
	}

	ntw, err := makeNetwork(conf, version)
	if err != nil {
		return fmt.Errorf("cannot build network: %w", err)
	}

	var (
		blocklist mtglib.IPBlocklist
		allowlist mtglib.IPBlocklist
		wg        sync.WaitGroup
		errChan   = make(chan error, 2)
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		bl, err := makeIPBlocklist(
			conf.Defense.Blocklist,
			logger.Named("blocklist"),
			ntw,
			func(ctx context.Context, size int) {
				eventStream.Send(ctx, mtglib.NewEventIPListSize(size, true))
			},
		)
		if err != nil {
			errChan <- fmt.Errorf("cannot build ip blocklist: %w", err)
			return
		}
		blocklist = bl
	}()

	go func() {
		defer wg.Done()
		al, err := makeIPAllowlist(
			conf.Defense.Allowlist,
			logger.Named("allowlist"),
			ntw,
			func(ctx context.Context, size int) {
				eventStream.Send(ctx, mtglib.NewEventIPListSize(size, false))
			},
		)
		if err != nil {
			errChan <- fmt.Errorf("cannot build ip allowlist: %w", err)
			return
		}
		allowlist = al
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		return err
	}

	opts := mtglib.ProxyOpts{
		Logger:          logger,
		Network:         ntw,
		AntiReplayCache: makeAntiReplayCache(conf),
		IPBlocklist:     blocklist,
		IPAllowlist:     allowlist,
		EventStream:     eventStream,

		Secret:             conf.Secret,
		DomainFrontingPort: conf.DomainFrontingPort.Get(mtglib.DefaultDomainFrontingPort),
		PreferIP:           conf.PreferIP.Get(mtglib.DefaultPreferIP),

		AllowFallbackOnUnknownDC: conf.AllowFallbackOnUnknownDC.Get(false),
		TolerateTimeSkewness:     conf.TolerateTimeSkewness.Value,
	}

	proxy, err := mtglib.NewProxy(opts)
	if err != nil {
		return fmt.Errorf("cannot create a proxy: %w", err)
	}

	listener, err := utils.NewListener(conf.BindTo.Get(""), 0)
	if err != nil {
		return fmt.Errorf("cannot start proxy: %w", err)
	}

	ctx := utils.RootContext()

	go proxy.Serve(listener)

	<-ctx.Done()
	listener.Close()
	proxy.Shutdown()

	return nil
}
