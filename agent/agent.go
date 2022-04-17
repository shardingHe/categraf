package agent

import (
	"fmt"
	"log"
	"path"
	"strings"

	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/inputs"
	"flashcat.cloud/categraf/pkg/cfg"
	"flashcat.cloud/categraf/types"
	"github.com/toolkits/pkg/file"

	// auto registry
	_ "flashcat.cloud/categraf/inputs/cpu"
	_ "flashcat.cloud/categraf/inputs/disk"
	_ "flashcat.cloud/categraf/inputs/diskio"
	_ "flashcat.cloud/categraf/inputs/kernel"
	_ "flashcat.cloud/categraf/inputs/linux_sysctl_fs"
	_ "flashcat.cloud/categraf/inputs/mem"
	_ "flashcat.cloud/categraf/inputs/net"
	_ "flashcat.cloud/categraf/inputs/netstat"
	_ "flashcat.cloud/categraf/inputs/redis"
	_ "flashcat.cloud/categraf/inputs/system"
)

type Agent struct {
	InputFilters map[string]struct{}
}

func NewAgent(filters map[string]struct{}) *Agent {
	return &Agent{
		InputFilters: filters,
	}
}

func (a *Agent) Start() {
	log.Println("I! agent starting")

	a.startInputs()
}

func (a *Agent) Stop() {
	log.Println("I! agent stopping")

	for name := range InputReaders {
		InputReaders[name].QuitChan <- struct{}{}
		close(InputReaders[name].Queue)
	}

	log.Println("I! agent stopped")
}

func (a *Agent) Reload() {
	log.Println("I! agent reloading")

	a.Stop()
	a.Start()
}

func (a *Agent) startInputs() error {
	names, err := a.getInputsByDirs()
	if err != nil {
		return err
	}

	if len(names) == 0 {
		log.Println("I! no inputs")
		return nil
	}

	for _, name := range names {
		if len(a.InputFilters) > 0 {
			// do filter
			if _, has := a.InputFilters[name]; !has {
				continue
			}
		}

		creator, has := inputs.InputCreators[name]
		if !has {
			log.Println("E! input:", name, "not supported")
			continue
		}

		// construct input instance
		instance := creator()

		// set configurations for input instance
		cfg.LoadConfigs(path.Join(config.Config.ConfigDir, "input."+name), instance)

		if err = instance.Init(); err != nil {
			log.Println("E! failed to init input:", name, "error:", err)
			continue
		}

		reader := &Reader{
			Instance: instance,
			QuitChan: make(chan struct{}, 1),
			Queue:    make(chan *types.Sample, config.Config.WriterOpt.ChanSize),
		}

		log.Println("I! input:", name, "started")
		reader.Start()

		InputReaders[name] = reader
	}

	return nil
}

// input dir should has prefix input.
func (a *Agent) getInputsByDirs() ([]string, error) {
	dirs, err := file.DirsUnder(config.Config.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get dirs under %s : %v", config.Config.ConfigDir, err)
	}

	count := len(dirs)
	if count == 0 {
		return dirs, nil
	}

	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if strings.HasPrefix(dirs[i], "input.") {
			names = append(names, dirs[i][6:])
		}
	}

	return names, nil
}
