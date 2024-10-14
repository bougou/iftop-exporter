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

	runPeriodic         bool
	runPeriodicInterval time.Duration
	runPeriodicDuration time.Duration

	debug bool
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

func (mgr *Manager) WithPeriodic(periodicInterval time.Duration, periodicDuration time.Duration) *Manager {
	mgr.runPeriodic = true
	mgr.runPeriodicInterval = periodicInterval
	mgr.runPeriodicDuration = periodicDuration
	return mgr
}

func (mgr *Manager) WithDebug(debug bool) *Manager {
	mgr.debug = debug
	return mgr
}

func (mgr *Manager) isStaticInterface(interfaceName string) bool {
	for _, name := range mgr.staticInterfaceNames {
		if name == interfaceName {
			return true
		}
	}

	return false
}

func (mgr *Manager) watch() error {
	if !mgr.dynamic {
		log.Println("dynamic not enabled")
		return nil
	}

	log.Println("dynamic enabled")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("dynamic enabled, create watcher failed, err: %s", err)
	}
	defer watcher.Close()

	log.Printf("start watch dynamic dir (%s)", mgr.dynamicDir)
	err = watcher.Add(mgr.dynamicDir)
	if err != nil {
		return fmt.Errorf("watch dynamic directory (%s) failed, err: %s", mgr.dynamicDir, err)
	}

	watchingFile := filepath.Join(mgr.dynamicDir, ".watching")
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

			if mgr.isStaticInterface(interfaceName) {
				log.Printf("watch ignored static interface (%s)", interfaceName)
				continue
			}

			log.Printf("check event operation for interface (%s)", interfaceName)
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Chmod) {
				log.Printf("[event (%s)] try to call LinkByName for interface (%s)", event.Op, interfaceName)
				_, err := netlink.LinkByName(interfaceName)
				if err != nil {
					if _, ok := err.(netlink.LinkNotFoundError); ok {
						log.Printf("interface ignored, not found link for interface (%s)", interfaceName)
					} else {
						log.Printf("call LinkByName failed, err: %s", err)
					}
					continue
				}

				interfaceInfo := map[string]string{}

				b, err := os.ReadFile(event.Name)
				if err != nil {
					log.Printf("read file failed for interface (%s), err: %s", interfaceName, err)
					continue
				}

				if err := json.Unmarshal(b, &interfaceInfo); err != nil {
					log.Printf("json unmarshal failed for interface (%s), err: %s", interfaceName, err)
					continue
				}

				mgr.lock.Lock()
				mgr.dynamicInterfaceInfo[interfaceName] = interfaceInfo
				mgr.lock.Unlock()

				owner := interfaceInfo["owner"]
				log.Printf("try to start iftop for interface (%s, %s)", interfaceName, owner)
				mgr.start(interfaceName)
				continue
			}

			if event.Has(fsnotify.Remove) {
				log.Printf("[event (%s)] try to stop iftop task for interface (%s)", event.Op, interfaceName)
				mgr.stop(interfaceName)
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

func (mgr *Manager) static() error {
	for _, interfaceName := range mgr.staticInterfaceNames {
		go mgr.exec(interfaceName)
	}
	return nil
}

func (mgr *Manager) start(interfaceName string) {
	go mgr.exec(interfaceName)
}

func (mgr *Manager) stop(interfaceName string) {
	// send remove signal
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	if removeCh, ok := mgr.removeChs[interfaceName]; ok {
		close(removeCh)
	}
}

func (mgr *Manager) exec(interfaceName string) error {
	// To avoid starting multiple iftop tasks for the same interface
	mgr.lock.Lock()
	_, exists := mgr.tasks[interfaceName]
	if exists {
		log.Printf("iftop task already there (%s)", interfaceName)
		mgr.lock.Unlock()
		return nil
	}
	iftopTask := mgr.newIftopTask(interfaceName)
	removeCh := make(chan int)
	exitCh := make(chan error)
	mgr.tasks[interfaceName] = iftopTask
	mgr.removeChs[interfaceName] = removeCh
	mgr.lock.Unlock()

	go func() {
		mgr.Debugf("initial iftop task start (%s)", interfaceName)
		err := iftopTask.Run()
		if err != nil {
			mgr.Debugf("initial iftop task exit (%s), err: %s", interfaceName, err)
		} else {
			mgr.Debugf("initial iftop task exit (%s)", interfaceName)
		}
		exitCh <- err
	}()

	for {
		select {

		case exitErr := <-exitCh:
			sleepSeconds := int(mgr.runPeriodicInterval.Seconds())
			if !mgr.runPeriodic {
				sleepSeconds = 2 // Just sleep 2 seconds for continuous mode
			}

			if exitErr != nil {
				log.Printf("iftop task exit (%s) with error (%s), wait several seconds and start again", interfaceName, exitErr)
			}

			if err := mgr.startTask(interfaceName, removeCh, exitCh, sleepSeconds); err != nil {
				log.Printf("start task failed, err: %s", err)
			}

		case <-removeCh:
			log.Printf("exec got remove signal for interface (%s)", interfaceName)
			if err := mgr.removeTask(interfaceName); err != nil {
				log.Printf("remove task failed, err: %s", err)
			}

			return nil
		}

	}
}

// startTask waits for specified sleepSeconds and start iftop task for specified interface.
func (mgr *Manager) startTask(interfaceName string, removeCh <-chan int, exitCh chan<- error, sleepSeconds int) error {
	select {
	case <-time.After(time.Duration(sleepSeconds) * time.Second):
		go func() {
			iftopTask := mgr.newIftopTask(interfaceName)

			if !mgr.runPeriodic {
				// for continuous mode: update the cached iftop task before iftop task run
				mgr.lock.Lock()
				mgr.tasks[interfaceName] = iftopTask
				mgr.lock.Unlock()
			}

			mgr.Debugf("iftop task start (%s)", interfaceName)
			err := iftopTask.Run()

			if mgr.runPeriodic {
				// for periodic mode: update the cached iftop task after iftop task exit
				mgr.lock.Lock()
				mgr.tasks[interfaceName] = iftopTask
				mgr.lock.Unlock()
			}

			if err != nil {
				log.Printf("iftop task failed (%s), err: %s", interfaceName, err)
			}
			exitCh <- err
		}()

		return nil

	case <-removeCh:
		log.Printf("start task got remove signal for interface (%s), no need to start", interfaceName)
		return nil
	}
}

func (mgr *Manager) removeTask(interfaceName string) error {
	iftopTask, ok := mgr.tasks[interfaceName]
	if !ok {
		return nil
	}

	log.Printf("remove task, try to kill iftop for interface (%s)", interfaceName)

	if iftopTask.GetCmd() != nil && iftopTask.GetCmd().Process != nil {
		if err := iftopTask.GetCmd().Process.Kill(); err != nil {
			log.Printf("kill process for interface (%s) failed, err: %s", interfaceName, err)
		} else {
			log.Printf("kill process for interface (%s) succeeded", interfaceName)
		}
	}

	mgr.lock.Lock()
	delete(mgr.removeChs, interfaceName)
	delete(mgr.tasks, interfaceName)
	delete(mgr.dynamicInterfaceInfo, interfaceName)
	mgr.lock.Unlock()
	return nil
}

func (mgr *Manager) updateMetricsLoop() error {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			mgr.Debugf("update metrics: found total (%d) iftop tasks", len(mgr.tasks))
			states := []iftop.State{}
			for _, iftopTask := range mgr.tasks {
				states = append(states, iftopTask.State())
			}
			mgr.updateMetrics(states)
		}
	}
}

func (mgr *Manager) Run() error {
	log.Println("start: static interfaces")
	mgr.static()
	go mgr.watch()

	// block here
	if err := mgr.updateMetricsLoop(); err != nil {
		return fmt.Errorf("manager update metrics loop failed, err: %s", err)
	}

	return nil
}

func (mgr *Manager) newIftopTask(interfaceName string) *iftop.Task {
	options := iftop.Options{
		InterfaceName:    interfaceName,
		NoHostnameLookup: true,
		SortBy:           iftop.SortBy2s,
	}

	if mgr.runPeriodic {
		options.SingleSeconds = int(mgr.runPeriodicDuration.Seconds())
	}

	return iftop.NewTask(options)
}
