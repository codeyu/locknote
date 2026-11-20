package sleepwatch

import (
	"testing"
	"time"
)

func TestSlept(t *testing.T) {
	s := time.Second
	cases := []struct {
		wall, mono time.Duration
		want       bool
	}{
		{5 * s, 5 * s, false},                // 正常
		{6 * s, 5 * s, false},                // 轻微误差
		{2 * time.Hour, 5 * s, true},         // 单调时钟暂停：典型的睡眠
		{40 * s, 5 * s, true},                // 差距超过 30 秒
		{2 * time.Hour, 2 * time.Hour, true}, // 单调时钟也走了（进程被挂起很久）
		{100 * s, 100 * s, true},             // 超过 90 秒才醒来
		{-time.Hour, 5 * s, false},           // 时钟往回调，不当作睡眠
	}
	for _, c := range cases {
		if got := Slept(c.wall, c.mono, DefaultGap, DefaultStall); got != c.want {
			t.Errorf("Slept(wall=%v, mono=%v) = %v, want %v", c.wall, c.mono, got, c.want)
		}
	}
}

func TestWatcherStartStop(t *testing.T) {
	w := Start(10*time.Millisecond, time.Hour, time.Hour, func() {
		t.Error("no sleep should be detected")
	})
	time.Sleep(50 * time.Millisecond)
	w.Stop()
	w.Stop()
	var nilWatcher *Watcher
	nilWatcher.Stop()
}

func TestWatcherCallsOnWakeWhenStalled(t *testing.T) {
	called := make(chan struct{}, 10)
	// stall 设为 0：任何一次检查都会被当成“醒来”，用来验证回调路径
	w := Start(5*time.Millisecond, time.Hour, 0, func() { called <- struct{}{} })
	defer w.Stop()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("onWake was not called")
	}
}
