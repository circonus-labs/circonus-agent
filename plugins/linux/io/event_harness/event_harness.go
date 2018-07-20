// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package event_harness

import "bufio"
import "path/filepath"
import "fmt"
import "io"
import "os"
import "os/signal"
import "time"

var base = "/sys/kernel/debug/tracing/instances"

func echoEmulate(path, val string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err2 := file.WriteString(val)
	return err2
}
func StartTracing(instance string, args [][]string) error {
	inst := filepath.Join(base, instance)
	if err := os.Mkdir(inst, 0550); err != nil {
		return err
	}
	for _, pa := range args {
		if err := echoEmulate(filepath.Join(inst, pa[0]), pa[1]); err != nil {
			return err
		}
	}
	if err := echoEmulate(filepath.Join(inst, "tracing_on"), "1\n"); err != nil {
		return err
	}

	return nil
}
func StopTracing(instance string) error {
	inst := filepath.Join(base, instance)
	echoEmulate(filepath.Join(inst, "tracing_on"), "0\n")
	err := os.Remove(inst)
	return err
}

func ProcessTrace(pipe *os.File, handler func(string), tasks chan func(), finished chan error) {
	defer pipe.Close()
	rdr := bufio.NewReader(pipe)
	// This stupid timeout is because of https://lkml.org/lkml/2014/6/10/30
	pipe.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	line, err := rdr.ReadString('\n')
	for err != io.EOF {
		if line != "" {
			handler(string(line))
		}
		select {
		case f := <-tasks:
			f()
		default:
		}
		pipe.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		line, err = rdr.ReadString('\n')
	}
	finished <- nil
}

type Harness struct {
	Done  chan error
	Tasks chan func()
}

func HarnessMain(instance string, args [][]string, handler func(string)) (*Harness, error) {
	complete := make(chan error)
	inline_tasks := make(chan func())
	done := make(chan error)
	inst := filepath.Join(base, instance)

	if err := StartTracing(instance, args); err != nil {
		StopTracing(instance)
		return nil, fmt.Errorf("Failed to start tracing: %s", err)
	}
	pipe, erro := os.Open(filepath.Join(inst, "trace_pipe"))
	if erro != nil {
		StopTracing(instance)
		return nil, fmt.Errorf("Failed to read trace: %s", erro)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			done <- nil
		}
	}()

	go func() {
		go ProcessTrace(pipe, handler, inline_tasks, done)
		err := <-done
		pipe.Close()
		close(done)
		StopTracing(instance)
		complete <- err
	}()
	return &Harness{
		Done:  complete,
		Tasks: inline_tasks,
	}, nil
}
