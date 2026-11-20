// Package sleepwatch 不依赖平台 API，检测电脑是否进入过睡眠 / 休眠。
//
// 原理：定时器每隔 interval 醒来一次，比较两种时钟走过的时间：
//   - 墙上时钟（wall clock）在睡眠期间照常前进；
//   - Go 的单调时钟在 Windows、macOS、Linux 上睡眠期间通常暂停。
//
// 两者差距超过 gap，或者本该 interval 后醒来却隔了 stall 以上才醒来（进程被挂起），
// 都视为发生过睡眠。手动大幅调整系统时间也会触发，这种情况下多锁一次是可以接受的。
package sleepwatch

import (
	"sync"
	"time"
)

const (
	DefaultInterval = 5 * time.Second
	DefaultGap      = 30 * time.Second
	DefaultStall    = 90 * time.Second
)

// Slept 根据一次检查中墙上时钟和单调时钟各自走过的时间，判断期间是否睡眠过。
func Slept(wallElapsed, monoElapsed, gap, stall time.Duration) bool {
	if wallElapsed-monoElapsed > gap {
		return true
	}
	return monoElapsed > stall || wallElapsed > stall
}

// Watcher 在后台检测睡眠，检测到时调用回调。
type Watcher struct {
	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// Start 启动后台检测。onWake 在检测到睡眠后（通常是刚唤醒时）调用。
func Start(interval, gap, stall time.Duration, onWake func()) *Watcher {
	w := &Watcher{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		last := time.Now()
		for {
			select {
			case <-w.stop:
				return
			case <-ticker.C:
				now := time.Now()
				mono := now.Sub(last)                   // 两个值都带单调时钟读数
				wall := now.Round(0).Sub(last.Round(0)) // Round(0) 去掉单调读数，只比较墙上时钟
				last = now
				if Slept(wall, mono, gap, stall) {
					onWake()
				}
			}
		}
	}()
	return w
}

// Stop 停止检测并等待后台协程退出。可以重复调用。
func (w *Watcher) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() { close(w.stop) })
	<-w.done
}
