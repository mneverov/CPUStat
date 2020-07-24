package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/docker"
	"github.com/shirou/gopsutil/process"
)

var global []byte

func main() {
	log.Printf("starting metrics\n\n")

	log.Printf("_SC_CLK_TCK=%d\n", GetUserHZ())
	log.Printf("cgroup mode=%q\n", GetCGroupMode())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		s := <-stop
		log.Printf("got signal %q\n", s)
		cancel()
	}()

	// start work load routine
	go doWork()
	// start CPU measurement routine
	go printCpuWithGopsutil()

	<-ctx.Done()
}

func printCpuWithGopsutil() {
	t := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-t.C:
			// cpu overall
			ps, err := cpu.Percent(0*time.Second, false)
			if err != nil {
				fmt.Printf("failed to get cpu percent %q\n", err)
			} else {
				fmt.Printf("cpu total %v\n", ps)
			}

			// per process
			printProcessStat()

			// CPU for metrics docker container
			containerID, err := findMetricsContainerID()
			if err != nil {
				fmt.Println(err)
			} else {
				go printDockerStat(containerID)
			}
		}
	}
}

func findMetricsContainerID() (string, error) {
	dStat, err := docker.GetDockerStat()
	if err != nil {
		return "", err
	}
	for _, s := range dStat {
		if s.Name == "metrics" {
			return s.ContainerID, nil
		}
	}

	return "", errors.New("could not find metrics container")
}

func printDockerStat(containerID string) {
	timeStat, err := docker.CgroupCPUDocker(containerID)
	if err != nil {
		fmt.Printf("failed to get time stat %q\n", err)
	} else {
		fmt.Printf("cgroup cpu docker time stat %q\n", timeStat)
	}

	cpuUsage, err := docker.CgroupCPUUsageDocker(containerID)
	if err != nil {
		fmt.Printf("failed to get cpu usage %q\n", err)
		return
	}
	fmt.Printf("docker cpu usage %f\n", cpuUsage)
}

func printProcessStat() {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		fmt.Printf("failed to get current process %q\n", err)
		return
	}
	cpu, err := p.CPUPercent()
	if err != nil {
		fmt.Printf("failed to get CPU percent %q\n", err)
		return
	}
	fmt.Printf("current process CPU usage %f\n", cpu)
}

func doWork() {
	rand.Seed(time.Now().UnixNano())
	producer := func() byte {
		return byte(rand.Intn(256))
	}

	for {
		s := createRandomSlice(1000000, producer)
		global = bubbleSort(s)
		// "use" the slice in case the compiler is smart enough to get rid of it
		fmt.Printf("%v", global)
	}
}

func createRandomSlice(size int, f func() byte) []byte {
	// do not use make with size -> more work
	res := []byte{}
	for i := 0; i < size; i++ {
		res = append(res, f())
	}
	return res
}

/*
CPU is measured with top
Without sleep the CPU consumption is 100%
With 1 Millisecond sleep it should consume ~ 70% CPU
*/

// bubbleSort sorts the given slice.
// Good enough to keep CPU busy.
func bubbleSort(s []byte) []byte {
	sorted := make([]byte, len(s))
	copy(sorted, s)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
		time.Sleep(1 * time.Millisecond)
	}
	return sorted
}
