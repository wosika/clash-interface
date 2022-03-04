package clash

import (
	"path/filepath"
	"time"

	"github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/hub/executor"
	"github.com/Dreamacro/clash/log"
	T "github.com/Dreamacro/clash/tunnel"
	"github.com/Dreamacro/clash/tunnel/statistic"
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/socks"
)

var (
	stack           core.LWIPStack
	trafficReceiver TrafficReceiver
	nativeLogger    NativeLogger
)

type PacketFlow interface {
	WritePacket(packet []byte)
}

type TrafficReceiver interface {
	ReceiveTraffic(up int64, down int64)
}

type NativeLogger interface {
	Log(level string, payload string)
}

func ReadPacket(data []byte) {
	stack.Write(data)
}

func Setup(flow PacketFlow, homeDir string, config string) error {
	constant.SetHomeDir(homeDir)
	constant.SetConfig("")
	cfg, err := executor.ParseWithBytes(([]byte)(config))
	if err != nil {
		return err
	}
	executor.ApplyConfig(cfg, true)
	go fetchLogs()
	stack = core.NewLWIPStack()
	core.RegisterTCPConnHandler(socks.NewTCPHandler("127.0.0.1", uint16(cfg.General.MixedPort)))
	core.RegisterUDPConnHandler(socks.NewUDPHandler("127.0.0.1", uint16(cfg.General.MixedPort), 30*time.Second))
	core.RegisterOutputFn(func(data []byte) (int, error) {
		flow.WritePacket(data)
		return len(data), nil
	})
	go fetchTraffic()
	return nil
}

func ApplyConfig(uuid string) error {
	if stack == nil {
		return nil
	}
	path := filepath.Join(constant.Path.HomeDir(), uuid, "config.yaml")
	cfg, err := executor.ParseWithPath(path)
	if err != nil {
		return err
	}
	constant.SetConfig(path)
	CloseAllConnections()
	cfg.General = executor.GetGeneral()
	executor.ApplyConfig(cfg, false)
	return nil
}

func SetTunnelMode(mode string) {
	if stack == nil {
		return
	}
	CloseAllConnections()
	T.SetMode(T.ModeMapping[mode])
}

func CloseAllConnections() {
	snapshot := statistic.DefaultManager.Snapshot()
	for _, c := range snapshot.Connections {
		c.Close()
	}
}

func SetTrafficReceiver(receive TrafficReceiver) {
	trafficReceiver = receive
}

func fetchTraffic() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	t := statistic.DefaultManager
	for range tick.C {
		if trafficReceiver == nil {
			continue
		}
		up, down := t.Now()
		trafficReceiver.ReceiveTraffic(up, down)
	}
}

func SetNativeLogger(logger NativeLogger) {
	nativeLogger = logger
}

func fetchLogs() {
	sub := log.Subscribe()
	defer log.UnSubscribe(sub)
	for elm := range sub {
		if nativeLogger == nil {
			continue
		}
		log := elm.(*log.Event)
		nativeLogger.Log(log.Type(), log.Payload)
	}
}
