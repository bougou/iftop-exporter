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

	runPeriodically     bool
	runPeriodicInterval time.Duration
	runPeriodicDuration time.Duration
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

func (manager *Manager) WithRunPeriodically(interval time.Duration, duration time.Duration) *Manager {
	manager.runPeriodically = true
	manager.runPeriodicInterval = interval
	manager.runPeriodicDuration = duration
	return manager
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

				owner := interfaceInfo["owner"]
				log.Printf("try to start iftop for interface (%s, %s)", interfaceName, owner)
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
	// To avoid starting multiple iftop tasks for the same interface
	manager.lock.Lock()
	_, exists := manager.tasks[interfaceName]
	if exists {
		log.Printf("iftop task already there (%s)", interfaceName)
		manager.lock.Unlock()
		return nil
	}
	iftopTask := manager.newIftopTask(interfaceName)
	removeCh := make(chan int)
	exitCh := make(chan error)
	manager.tasks[interfaceName] = iftopTask
	manager.removeChs[interfaceName] = removeCh
	manager.lock.Unlock()

	go func() {
		log.Printf("initial iftop task start (%s)", interfaceName)
		err := iftopTask.Run()
		if err != nil {
			log.Printf("initial iftop task exit (%s), err: %s", interfaceName, err)
		}
		exitCh <- err
	}()

	for {
		select {
		case <-exitCh:
			go func() {

				if manager.runPeriodically {
					log.Printf("iftop task exit (%s), wait periodic interval (%s) and start again", interfaceName, manager.runPeriodicInterval)
					time.Sleep(manager.runPeriodicInterval)

					iftopTask := manager.newIftopTask(interfaceName)
					log.Printf("iftop task start (%s)", interfaceName)
					err := iftopTask.Run()
					if err != nil {
						log.Printf("iftop task failed (%s), err: %s", interfaceName, err)
						exitCh <- err
						return
					}

					// For periodic mode,ONLY update the task cached in manager.tasks if iftop task exit without error
					manager.tasks[interfaceName] = iftopTask
					exitCh <- err

				} else {
					log.Printf("iftop task exit (%s), try start again", interfaceName)
					time.Sleep(2 * time.Second)

					// For continuous mode, immediately update the cached iftop Task, the task is long-running, no need to wait for it exit.
					iftopTask := manager.newIftopTask(interfaceName)
					manager.tasks[interfaceName] = iftopTask

					log.Printf("iftop task start (%s)", interfaceName)
					err := iftopTask.Run()
					if err != nil {
						log.Printf("iftop task failed (%s), err: %s", interfaceName, err)
					}
					exitCh <- err
				}
			}()

		case <-removeCh:
			if err := manager.removeTask(interfaceName); err != nil {
				log.Printf("remove task failed, err: %s", err)
				return nil
			}
		}

	}
}

func (manager *Manager) removeTask(interfaceName string) error {
	iftopTask, ok := manager.tasks[interfaceName]
	if !ok {
		return nil
	}

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
}

func (manager *Manager) updateMetricsLoop() error {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			taskIDs := []string{}
			for _, iftopTask := range manager.tasks {
				taskIDs = append(taskIDs, iftopTask.ID())
			}
			log.Printf("update metrics: found total (%d) iftop tasks (%v)", len(manager.tasks), taskIDs)

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

func (manager *Manager) newIftopTask(interfaceName string) *iftop.Task {
	options := iftop.Options{
		InterfaceName:    interfaceName,
		NoHostnameLookup: true,
		SortBy:           iftop.SortBy2s,
	}

	if manager.runPeriodically {
		options.SingleSeconds = int(manager.runPeriodicDuration.Seconds())
	}

	return iftop.NewTask(options)
}
