package docker

import (
	"fmt"
	"sync"

	"github.com/baidu/openedge/logger"
	"github.com/baidu/openedge/master/engine"
	openedge "github.com/baidu/openedge/sdk/openedge-go"
	"github.com/orcaman/concurrent-map"
)

const (
	fmtVolume   = "%s:/%s"
	fmtVolumeRO = "%s:/%s:ro"
)

type dockerService struct {
	cfg       openedge.ServiceInfo
	params    containerConfigs
	engine    *dockerEngine
	instances cmap.ConcurrentMap
	log       logger.Logger
}

func (s *dockerService) Name() string {
	return s.cfg.Name
}

func (s *dockerService) Stats() openedge.ServiceStatus {
	instances := s.instances.Items()
	results := make(chan openedge.InstanceStatus, len(instances))

	var wg sync.WaitGroup
	for _, v := range instances {
		wg.Add(1)
		go func(i *dockerInstance, wg *sync.WaitGroup) {
			defer wg.Done()
			results <- i.State()
		}(v.(*dockerInstance), &wg)
	}
	wg.Wait()
	close(results)
	r := openedge.NewServiceStatus(s.cfg.Name)
	for i := range results {
		r.Instances = append(r.Instances, i)
	}
	return r
}

func (s *dockerService) Start() error {
	s.log.Debugf("%s replica: %d", s.cfg.Name, s.cfg.Replica)
	var instanceName string
	for i := 0; i < s.cfg.Replica; i++ {
		if i == 0 {
			instanceName = s.cfg.Name
		} else {
			instanceName = fmt.Sprintf("%s.i%d", s.cfg.Name, i)
		}
		err := s.startInstance(instanceName, nil)
		if err != nil {
			s.Stop()
			return err
		}
	}
	return nil
}

func (s *dockerService) Stop() {
	var wg sync.WaitGroup
	for _, v := range s.instances.Items() {
		wg.Add(1)
		go func(i *dockerInstance, wg *sync.WaitGroup) {
			defer wg.Done()
			i.Close()
		}(v.(*dockerInstance), &wg)
	}
	wg.Wait()
}

func (s *dockerService) ReportInstance(instanceName string, stats map[string]interface{}) error {
	i, ok := s.instances.Get(instanceName)
	if !ok {
		s.log.Debugf("instance (%s) not found", instanceName)
		return nil
	}
	i.(*dockerInstance).SetStats(stats)
	return nil
}

func (s *dockerService) StartInstance(instanceName string, dynamicConfig map[string]string) error {
	return s.startInstance(instanceName, dynamicConfig)
}

func (s *dockerService) startInstance(instanceName string, dynamicConfig map[string]string) error {
	s.StopInstance(instanceName)
	params := s.params
	params.config.Env = engine.GenerateInstanceEnv(instanceName, s.params.config.Env, dynamicConfig)
	i, err := s.newInstance(instanceName, params)
	if err != nil {
		return err
	}
	s.instances.Set(instanceName, i)
	return nil
}

func (s *dockerService) StopInstance(instanceName string) error {
	i, ok := s.instances.Get(instanceName)
	if !ok {
		s.log.Debugf("instance (%s) not found", instanceName)
		return nil
	}
	return i.(*dockerInstance).Close()
}
