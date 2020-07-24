# Tracking CPU Usage

This repository is my attempt to understand how CPU is measured by different tools.

#

  - [Run](#run)
  - [Metrics App](#metrics-app)
  - [gopsutil](#gopsutil)
    - [CPU Overall](#cpu-overall)
    - [Per Process](#per-process)
      - [bootTime](#boottime)
      - [starttime](#starttime)
      - [utime](#utime)
      - [stime](#stime)
      - [iotime](#iotime)
      - [_SC_CLK_TCK](#_sc_clk_tck)
    - [CgroupCPUUsageDocker](#cgroupcpuusagedocker)
    - [CgroupCPUDocker](#cgroupcpudocker)
  - [top](#top)
  - [docker stats](#docker-stats)
    - [The Docker CLI](#the-docker-cli)
    - [Dockerd](#dockerd)
    - [Containerd](#containerd)
  - [Cadvisor](#cadvisor)


## Run

To start the metrics app in docker run:

```
docker-compose up -d --build
```

It will build and run the app in docker as well as cadvisor container.

To check the metrics app logs run:

```
docker logs -f metrics
```

The output should look like:

```
cpu total [46.368715083911184]
current process CPU usage 78.888166
docker not available
```

From gopsutil:

`cpu total` is the `the percentage of cpu used either per CPU or combined`

and

`current process CPU usage` shows `how many percent of the CPU time this process uses`.

The last line is only relevant when the metrics app is running in docker, and another
instance is running on the host. To do that run the app the usual way:

```
go run .
```

The output should look like:

```
starting metrics

_SC_CLK_TCK=100
cgroup mode="Hybrid"
cpu total [42.3698384199126]
current process CPU usage 66.571980
cgroup cpu docker time stat "{\"cpu\":\"some_id\",\"user\":559.9,\"system\":81.0,\"idle\":0.0,\"nice\":0.0,\"iowait\":0.0,\"irq\":0.0,\"softirq\":0.0,\"steal\":0.0,\"guest\":0.0,\"guestNice\":0.0}"
docker cpu usage 683.887043
```

`_SC_CLK_TCK=100` - see corresponding section for [SC_CLK_TCK](#_SC_CLK_TCK) below.

`cgroup mode="Hybrid"` - see corresponding section in [Containerd](#containerd).

Last two rows are explained in gopsutil section.

To check the cadvisor open one of the containers [on the cadvisor page](http://localhost:8080/containers/docker).


## Metrics App

The application generates some predictable constant CPU load by creating a random byte slice of size 1_000_000 and
sorting it with bubble sort. On my machine the app consumes ~70% CPU.

In the app I used gopsutil to measure overall CPU load, the CPU consumed by the app process (per one CPU), and to print
the available CPU statistics for the application running in Docker.


## gopsutil

### CPU Overall

gopsutil parses the cpu line of the `/proc/stat` file. To calculate the CPU % it uses the following formula:

```
busy := t.User + t.System + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal
all := busy + t.Idle
percent := math.Min(100, math.Max(0, (t2Busy-t1Busy)/(t2All-t1All)*100))
```

which should correspond to top Line 2:
```
 Line 2 shows CPU state percentages based on the interval since the last refresh.
```

### Per Process

Let's have some background for the calculation (see `man proc`).

##### bootTime
bootTime is read from `/proc/stat` file:  
boot time, in seconds since the Epoch

All other measurements are read from `/proc/{pid}/stat` file.

##### starttime
(22) starttime  %llu The time the process started after system boot.

The value is expressed in clock ticks (divide by sysconf(_SC_CLK_TCK)).

##### utime
(14) utime  %lu Amount of time that this process has been scheduled in user mode, measured in clock
ticks (divide by sysconf(_SC_CLK_TCK)). This includes guest time, guest_time (time spent running a
virtual CPU, see below), so that applications that are not aware of the guest time field do not
lose that time from their calculations.

##### stime
(15) stime  %lu Amount of time that this process has been scheduled in kernel mode, measured in clock
ticks (divide by sysconf(_SC_CLK_TCK)).

##### iotime
from the gopsutil comment:

There is no such thing as iotime in stat file. As an approximation, we will use delayacct_blkio_ticks
(42) delayacct_blkio_ticks  %llu  Aggregated block I/O delays, measured in clock ticks (centiseconds).

##### _SC_CLK_TCK
The number of clock ticks per second (see `man 7 time`).

**The software clock, HZ, and jiffies**

The accuracy of various system calls that set timeouts, and measure CPU time is limited by the
resolution of the software clock, a clock maintained by the kernel which measures time in jiffies.
The size of a jiffy is determined by the value of the kernel constant **HZ**.

The value of HZ varies across kernel versions and hardware platforms (100 - 1000). To find HZ configured run
```
grep 'CONFIG_HZ=' /boot/config-$(uname -r)
```

The times(2) system call is a special case. It reports times with a granularity defined by the kernel
constant **USER_HZ**. User-space applications can determine the value of this constant using sysconf(_SC_CLK_TCK).

The value can be found by running `getconf CLK_TCK`, and it is currently the same as the value of
sysconf(_SC_CLK_TCK); however, new applications should call sysconf() because the CLK_TCK macro may be withdrawn
in a future issue.

The metrics app uses [user_hz](user_hz.go) to print the USER_HZ value. It is safe to hardcode it to 100, see
[this PR](https://github.com/containerd/cgroups/pull/12), cgo function was added just for fun.

With these values in hand gopsutil calculates the CPU usage as following:

```
createTime := ((startTime / SC_CLK_TCK) + bootTime) * 1000
totalTime := time.Since(createTime).Seconds()

user := utime / SC_CLK_TCK
system := stime / SC_CLK_TCK
iowait := iotime / SC_CLK_TCK
cpuTotal := user + system + iowait

cpu := 100 * cpuTotal / totalTime
```

CPU per process by gopsutil is in sync with the measurements by top.

### CgroupCPUUsageDocker

CgroupCPUUsageDocker value is read from `/sys/fs/cgroup/cpuacct/docker/container_id/cpuacct.usage` and is provided in seconds.

### CgroupCPUDocker

CgroupCPUDocker is read from `/sys/fs/cgroup/cpuacct/docker/container_id/cpuacct.stat` and is provided in seconds:

```
cat /sys/fs/cgroup/cpuacct/docker/container_id/cpuacct.stat
user 133574
system 18070
```

From [docker dock](https://docs.docker.com/config/containers/runmetrics/)
```
For each container, a pseudo-file cpuacct.stat contains the CPU usage accumulated by the processes of the container,
broken down into user and system time. The distinction is:

user time is the amount of time a process has direct control of the CPU, executing process code.
system time is the time the kernel is executing system calls on behalf of the process.
Those times are expressed in ticks of 1/100th of a second, also called “user jiffies”.
```

## top

> top

top works well except when run in docker. Alpine linux image has top already installed. To check it run:

```
docker exec -ti metrics top
```

It shows much lower CPU than `docker stats` or `cadvisor`. Let's find out what these numbers are.

To calculate CPU top needs `utime` and `stime`, and top reads these values in the [stat2proc function](https://gitlab.com/procps-ng/procps/-/blob/master/proc/readproc.c#L591).

Then, `pcpu` [is calculated](https://gitlab.com/procps-ng/procps/-/blob/master/top/top.c#L2734), which is the number of
elapsed ticks from the previous measurement:

```
pcpu = utime + stime - (prev_frame.utime + prev_frame.stime)
```

We also need to know the elapsed time since the previous measurement. Top uses [uptime](https://gitlab.com/procps-ng/procps/-/blob/master/top/top.c#L2677) for that.
The rationales behind is that the time difference calculation using wallclock can return a negative value when the system is changed.
See the corresponding patch for more info ([1](https://www.freelists.org/post/procps/PATCH-top-use-clock-gettime-instead-of-gettimeofday),
[2](https://www.freelists.org/post/procps/PATCH-top-use-clock-gettime-instead-of-gettimeofday,2),
[3](https://www.freelists.org/post/procps/PATCH-top-use-clock-gettime-instead-of-gettimeofday,11)).

Now we need to [scale elapsed time](https://gitlab.com/procps-ng/procps/-/blob/master/top/top.c#L2682):

```
Frame_etscale = 100 / (Hertz * et * (Rc.mode_irixps ? 1 : smp_num_cpus))
```

So the final formula is:

```
cpu = pcpu * 100 / (Hertz * et * (Rc.mode_irixps ? 1 : smp_num_cpus))
```

`pcpu` is measured in ticks so `pcpu / Hertz` gives us seconds a CPU spent on this process, where the `Hertz` number is
explained in [_SC_CLK_TCK](#_sc_clk_tck).  

Dividing this number by `et` gives us the ratio of this process CPU time to the total elapsed time. We want to have percentage,
that is why we multiply the result by 100. 

Finally, when the Irix mode is off, we divide the result by the number of all CPUs available. From `man top`: 
You toggle Irix/Solaris modes with the `I' interactive command.

I wrote a [small script](top.sh) to reproduce the numbers from top. It expects a process PID as the first argument and
when no arguments provided it does the calculation for the process with PID 1. To execute it for the metrics app in docker run:    

```
docker exec -ti metrics /metrics/top.sh
```

The output should look like:

```
%CPU=80.1
scaled=10.0
```

where `%CPU=80.1` shows the CPU usage per one core, and `scaled=10.0` shows the CPU usage scaled per all cores.

So the top in linux alpine shows numbers in Solaris mode by default. Let's ssh to the container and check version:

```
docker exec -ti metrics /bin/sh
top -v
```

The result is: 

```
top: unrecognized option: v
BusyBox v1.31.1 () multi-call binary.
```

Ok, let's install procps and run top:

```
apk add procps
top
```

Now you should see CPU% for the process with PID 1 something around 70%.


## docker stats

cli -> dockerd -> containerd

### The Docker CLI

Cli is responsible for the printing docker container statistics `docker stats`.

It uses [container stats client](https://github.com/docker/cli/blob/master/cli/command/container/stats_helpers.go#L70)
to get metrics from the docker engine API. The client queries `/containers/id/stats` endpoint, see the corresponding code
in [ContainerStats](https://github.com/moby/moby/blob/master/client/container_stats.go#L19).

To get the same response run:

```
curl -v --unix-socket /var/run/docker.sock http://localhost/containers/metrics/stats
```

The simplified [function for the CPU% calculation](https://github.com/docker/cli/blob/master/cli/command/container/stats_helpers.go#L166)
looks like that:

```
func calculateCPUPercentUnix(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
    // calculate the change for the cpu usage of the container in between readings
    cpuDelta = float64(v.CPUStats.CPUUsage.TotalUsage) - float64(previousCPU)

    // calculate the change for the entire system between readings
    systemDelta = float64(v.CPUStats.SystemUsage) - float64(previousSystem)

    onlineCPUs = float64(v.CPUStats.OnlineCPUs)

    cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0

    return cpuPercent
}
```

Let's find out what `CPUStats.CPUUsage.TotalUsage` and `CPUStats.SystemUsage` are.

### Dockerd

Docker runs [the routine](https://github.com/moby/moby/blob/master/daemon/stats/collector.go#L83) to collect stats from
the supervisor indefinitely.

From [getSystemCPUUsage](https://github.com/moby/moby/blob/master/daemon/stats/collector_unix.go#L31) we can see that the
host system cpu usage (`SystemUsage`) calculation is based on `cpu` line in `/proc/stat` file.
The result is the amount of time in nanoseconds.

Other statistics come from containerd, and the `TotalUsage` is populated by
[stats.CPU.Usage.Total](https://github.com/moby/moby/blob/master/daemon/daemon_unix.go#L1408).

### Containerd

Containerd [provides handler](https://github.com/containerd/containerd/blob/master/api/services/tasks/v1/tasks.pb.go#L1806)
for the metrics endpoint, which calls [cgroup library](https://github.com/containerd/cgroups/blob/master/cgroup.go#L250)
to get CPU, memory and other statistics. The `stats.CPU.Usage.Total` comes from [cpuacct.go](https://github.com/containerd/cgroups/blob/master/cpuacct.go#L51).
The function reads a `cpuacct.usage` file with some given path. What is this path and where it comes from?

A cgroup for a container defines the path where the container stat files are located. [This function](statfs.go) will
print the current cgroup mode. I will focus on `hybrid` cgroup mode since I have it.

The path is combined of several pieces. To get the root the `/proc/self/mountinfo` [file is parsed](https://github.com/containerd/cgroups/blob/master/v1.go#L49).
Then, the result (`/sys/fs/cgroup/`) is joined with the name of a cgroup subsystem (it is `cpuacct` in our case).
For the next part we need to find out the metrics container PID. One way to do that is to run `docker top metrics`. With
this PID in hand we can get the last part of the path by parsing `/proc/PID/cgroup` file as it done [here](https://github.com/containerd/cgroups/blob/master/paths.go#L51)

```
cat /proc/PID/cgroup
...
6:cpu,cpuacct:/docker/some_container_id
...
```
i.e. the full path looks like `/sys/fs/cgroup/cpuacct/docker/container_id/cpuacct.usage`

That is where we get the `TotalUsage` for the calculation above.

## Cadvisor

Run the following commands to get the full info about the metrics container.

```
curl http://localhost:8080/api/v1.2/docker/metrics
or
curl http://localhost:8080/api/v2.1/stats/docker/full_docker_container_id
```

To get full container id run `docker ps --no-trunc`.

On start, cadvisor runs the housekeeping routine where on each housekeeping tick [the statistics are updated](https://github.com/google/cadvisor/blob/master/manager/container.go#L546). 

Since the container is run in docker [dockerContainerHandler](https://github.com/google/cadvisor/blob/master/container/docker/handler.go#L422)
will be used.

Cadvisor uses [runc](https://github.com/opencontainers/runc/blob/master/libcontainer/cgroups/fs/cpuacct.go#L65) to get
CPU usage. The path is the same as in docker calculations: `/sys/fs/cgroup/cpuacct/docker/container_id/cpuacct.usage`.

The path is created on start when registering a [root container](https://github.com/google/cadvisor/blob/master/container/docker/factory.go#L342).
To do that `/proc/self/mountinfo` and `/proc/self/cgroup` files are parsed, the data merged, and as the result one of the
subsystems `cpuacct` has a mountpoint `/sys/fs/cgroup/cpu,cpuacct`.
To find a container name all the paths from the previous step are traversed, and the found directories are made into container
references, that's where `/docker/container_id` comes from.  

The actual calculation happens on [UI](https://github.com/google/cadvisor/blob/master/cmd/internal/pages/assets/js/containers.js#L229)
and its quite simple:
 
```
(cur.cpu.usage.total - prev.cpu.usage.total) / intervalNs)
```                  

The time for the `intervalNs` calculation is not as elaborated as in `docker stats` or `top`, but just go standard `time.Now()`.  