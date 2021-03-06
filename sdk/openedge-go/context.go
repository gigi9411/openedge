package openedge

import (
	fmt "fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/baidu/openedge/logger"
	"github.com/baidu/openedge/protocol/mqtt"
	"github.com/baidu/openedge/utils"
)

// Env variable keys
const (
	EnvHostOSKey                 = "OPENEDGE_HOST_OS"
	EnvMasterAPIKey              = "OPENEDGE_MASTER_API"
	EnvRunningModeKey            = "OPENEDGE_RUNNING_MODE"
	EnvServiceNameKey            = "OPENEDGE_SERVICE_NAME"
	EnvServiceTokenKey           = "OPENEDGE_SERVICE_TOKEN"
	EnvServiceAddressKey         = "OPENEDGE_SERVICE_ADDRESS"
	EnvServiceInstanceNameKey    = "OPENEDGE_SERVICE_INSTANCE_NAME"
	EnvServiceInstanceAddressKey = "OPENEDGE_SERVICE_INSTANCE_ADDRESS"
)

const (
	// AppConfFileName application config file name
	AppConfFileName = "application.yml"
	// DefaultSockFile sock file of openedge by default
	DefaultSockFile = "/var/run/openedge.sock"
	// DefaultPidFile pid file of openedge by default
	DefaultPidFile = "/var/run/openedge.pid"
	// DefaultConfFile config path of the service by default
	DefaultConfFile = "etc/openedge/service.yml"
	// DefaultDBDir db dir of the service by default
	DefaultDBDir = "var/db/openedge"
	// DefaultRunDir  run dir of the service by default
	DefaultRunDir = "var/run/openedge"
	// DefaultLogDir  log dir of the service by default
	DefaultLogDir = "var/log/openedge"
)

// Context of service
type Context interface {
	// returns the system configuration of the service, such as hub and logger
	Config() *ServiceConfig
	// loads the custom configuration of the service
	LoadConfig(interface{}) error
	// creates a Client that connects to the Hub through system configuration,
	// you can specify the Client ID and the topic information of the subscription.
	NewHubClient(string, []mqtt.TopicInfo) (*mqtt.Dispatcher, error)
	// returns logger interface
	Log() logger.Logger
	// waiting to exit, receiving SIGTERM and SIGINT signals
	Wait()
	// returns wait channel
	WaitChan() <-chan os.Signal

	// Master RESTful API

	// updates system and
	UpdateSystem(string, bool) error
	// inspects system stats
	InspectSystem() (*Inspect, error)
	// gets an available port of the host
	GetAvailablePort() (string, error)
	// reports the stats of the instance of the service
	ReportInstance(stats map[string]interface{}) error
	// starts an instance of the service
	StartInstance(serviceName, instanceName string, dynamicConfig map[string]string) error
	// stop the instance of the service
	StopInstance(serviceName, instanceName string) error
}

type ctx struct {
	sn  string // service name
	in  string // instance name
	cli *Client
	cfg ServiceConfig
	log logger.Logger
}

func newContext() (*ctx, error) {
	var cfg ServiceConfig
	err := utils.LoadYAML(DefaultConfFile, &cfg)
	if err != nil {
		return nil, err
	}
	sn, ok := os.LookupEnv(EnvServiceNameKey)
	if !ok {
		sn = "<unknown>"
	}
	in, ok := os.LookupEnv(EnvServiceInstanceNameKey)
	if !ok {
		in = "<unknown>"
	}
	log, err := logger.InitLogger(&cfg.Logger, "service", sn, "instance", in)
	if err != nil {
		return nil, err
	}

	c := &ctx{
		sn:  sn,
		in:  in,
		cfg: cfg,
		log: log,
	}
	c.cli, err = NewEnvClient()
	if err != nil {
		log.Warnln(err.Error())
	}
	return c, nil
}

func (c *ctx) NewHubClient(cid string, subs []mqtt.TopicInfo) (*mqtt.Dispatcher, error) {
	if c.cfg.Hub.Address == "" {
		return nil, fmt.Errorf("hub not configured")
	}
	cc := c.cfg.Hub
	if cid != "" {
		cc.ClientID = cid
	}
	if subs != nil {
		cc.Subscriptions = subs
	}
	return mqtt.NewDispatcher(cc, c.log.WithField("cid", cid)), nil
}

func (c *ctx) LoadConfig(cfg interface{}) error {
	return utils.LoadYAML(DefaultConfFile, cfg)
}

func (c *ctx) Config() *ServiceConfig {
	return &c.cfg
}

func (c *ctx) Log() logger.Logger {
	return c.log
}

func (c *ctx) Wait() {
	<-c.WaitChan()
}

func (c *ctx) WaitChan() <-chan os.Signal {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	signal.Ignore(syscall.SIGPIPE)
	return sig
}

// InspectSystem inspect all stats
func (c *ctx) InspectSystem() (*Inspect, error) {
	return c.cli.InspectSystem()
}

// UpdateSystem updates and reloads config
func (c *ctx) UpdateSystem(file string, clean bool) error {
	return c.cli.UpdateSystem(file, clean)
}

// GetAvailablePort gets available port
func (c *ctx) GetAvailablePort() (string, error) {
	return c.cli.GetAvailablePort()
}

// ReportInstance reports the stats of the instance of the service
func (c *ctx) ReportInstance(stats map[string]interface{}) error {
	return c.cli.ReportInstance(c.sn, c.in, stats)
}

// StartInstance starts a new service instance with dynamic config
func (c *ctx) StartInstance(serviceName, instanceName string, dynamicConfig map[string]string) error {
	return c.cli.StartInstance(serviceName, instanceName, dynamicConfig)
}

// StopInstance stops a service instance
func (c *ctx) StopInstance(serviceName, instanceName string) error {
	return c.cli.StopInstance(serviceName, instanceName)
}
