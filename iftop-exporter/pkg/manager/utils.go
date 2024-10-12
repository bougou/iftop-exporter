package manager

import "log"

func (mgr *Manager) Debug(v ...any) {
	if mgr.debug {
		log.Println(v...)
	}
}

func (mgr *Manager) Debugf(format string, v ...any) {
	if mgr.debug {
		log.Printf(format, v...)
	}
}
