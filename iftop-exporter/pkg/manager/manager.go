package manager

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bougou/iftop-exporter/iftop-exporter/pkg/iftop"
	"github.com/fsnotify/fsnotify"
	"github.com/vishvananda/netlink"
)

// Manager manages how to start/stop iftop tasks for specified interfaces, and
// how to update prometheus metrics by interpreting iftop state.
type Manager struct {
	tasks     map[string]*iftop.Task // key is interfaceName
	removeChs map[string]chan int    // key is interfaceName
	lock      sync.Mutex

	staticInterfaceNames []string
	dynamic              bool
	dynamicDir           string
	dynamicInterfaceInfo map[string]map[string]string // labels for each interfaceName
}

func NewManager(staticIntefaceNames []string, dynamic bool, dynamicDir string) (*Manager, error) {
	manager := &Manager{
		tasks:     make(map[string]*iftop.Task),
		removeChs: make(map[string]chan int),

		staticInterfaceNames: staticIntefaceNames,
		dynamic:              dynamic,
		dynamicDir:           dynamicDir,
		dynamicInterfaceInfo: make(map[string]map[string]string),
	}

	return manager, nil
}

func (manager *Manager) isStaticInterface(interfaceName string) bool {
	for _, name := range manager.staticInterfaceNames {
		if name == interfaceName {
			return true
		}
	}

	return false
}

func (manager *Manager) watch() error {
	if !manager.dynamic {
		log.Println("dynamic not enabled")
		return nil
	}

	log.Println("dynamic enabled")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("dynamic enabled, create watcher failed, err: %s", err)
	}
	defer watcher.Close()

	log.Printf("start watch dynamic dir (%s)", manager.dynamicDir)
	err = watcher.Add(manager.dynamicDir)
	if err != nil {
		return fmt.Errorf("watch dynamic directory (%s) failed, err: %s", manager.dynamicDir, err)
	}

	watchingFile := filepath.Join(manager.dynamicDir, ".watching")
	if err := os.WriteFile(watchingFile, []byte(""), os.ModePerm); err != nil {
		return fmt.Errorf("create watching file (%s) failed, err: %s", watchingFile, err)
	}
	log.Printf("create watching file (%s) succeeded", watchingFile)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Println("not ok")
				return nil
			}

			log.Println("watch got event:", event)
			interfaceName := filepath.Base(event.Name)
			log.Println("watch got file name:", interfaceName)

			if interfaceName == ".watching" {
				log.Printf("watch ignored watching file")
				continue
			}

			if manager.isStaticInterface(interfaceName) {
				log.Printf("watch ignored static interface (%s)", interfaceName)
				continue
			}

			log.Printf("try to call LinkByName for interface (%s)", interfaceName)
			_, err := netlink.LinkByName(interfaceName)
			if err != nil {
				if _, ok := err.(netlink.LinkNotFoundError); ok {
					log.Printf("interface ignored, not found link for interface (%s)", interfaceName)
				}
				log.Printf("call LinkByName failed, err: %s", err)
				continue
			}

			log.Printf("check event operation for interface (%s)", interfaceName)
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Chmod) {
				interfaceInfo := map[string]string{}

				b, err := os.ReadFile(event.Name)
				if err != nil {
					log.Printf("read file failed, err: %s", err)
				} else {
					if err := json.Unmarshal(b, &interfaceInfo); err != nil {
						log.Printf("json unmarshal failed, err: %s", err)
					} else {
						manager.lock.Lock()
						manager.dynamicInterfaceInfo[interfaceName] = interfaceInfo
						manager.lock.Unlock()
					}
				}

				manager.start(interfaceName)
				continue
			}

			if event.Has(fsnotify.Remove) {
				manager.stop(interfaceName)
				continue
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return err
			}
			log.Println("error:", err)
		}
	}
}

func (manager *Manager) start(interfaceName string) {
	if _, ok := manager.tasks[interfaceName]; ok {
		log.Printf("iftop task already started for interface (%s)", interfaceName)
		return
	}

	go manager.exec(interfaceName)
}

func (manager *Manager) stop(interfaceName string) {
	// send remove signal
	manager.lock.Lock()
	defer manager.lock.Unlock()

	if removeCh, ok := manager.removeChs[interfaceName]; ok {
		close(removeCh)
	}
}

func (manager *Manager) static() error {
	for _, interfaceName := range manager.staticInterfaceNames {
		go manager.exec(interfaceName)
	}
	return nil
}

func (manager *Manager) exec(interfaceName string) error {

	var iftopTask *iftop.Task
	removeCh := make(chan int)
	exitCh := make(chan error)
	manager.removeChs[interfaceName] = removeCh

	var startTask = func(override bool) {
		manager.lock.Lock()

		if !override {
			if _, ok := manager.tasks[interfaceName]; ok {
				log.Printf("iftop task already there")
				return
			}
		}

		iftopTask = iftop.NewTask(interfaceName)
		manager.tasks[interfaceName] = iftopTask
		manager.lock.Unlock()

		go func() {
			err := iftopTask.Run()
			if err != nil {
				log.Printf("iftop task exit, err: %s", err)
			}
			exitCh <- err
		}()
	}

	startTask(false)

	for {
		select {
		case <-removeCh:
			if iftopTask.GetCmd() != nil && iftopTask.GetCmd().Process != nil {
				if err := iftopTask.GetCmd().Process.Kill(); err != nil {
					log.Printf("kill process for interface (%s) failed, err: %s", interfaceName, err)
				} else {
					log.Printf("kill process for interface (%s) succeeded", interfaceName)
				}
			}

			manager.lock.Lock()
			delete(manager.removeChs, interfaceName)
			delete(manager.tasks, interfaceName)
			manager.lock.Unlock()
			return nil

		case <-exitCh:
			log.Printf("iftop process for interface (%s) exit, try start again", interfaceName)
			time.Sleep(2 * time.Second)
			startTask(true)
		}
	}
}

func (manager *Manager) updateMetricsLoop() error {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Printf("update metrics: got (%d) iftop tasks", len(manager.tasks))

			states := []iftop.State{}
			for _, iftopTask := range manager.tasks {
				states = append(states, iftopTask.State())
			}
			manager.updateMetrics(states)
		}
	}
}

func (manager *Manager) Run() error {
	log.Println("start: static interfaces")
	manager.static()
	go manager.watch()

	// block here
	if err := manager.updateMetricsLoop(); err != nil {
		return fmt.Errorf("manager update metrics loop failed, err: %s", err)
	}

	return nil
}
