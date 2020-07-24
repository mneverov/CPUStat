#!/bin/sh

PID=$1
PID=${PID:-1}
SLEEP_SEC=3
CPUS=$(awk '/^cpu[0-9]+/ {n+=1}END {print n}' /proc/stat)
# see explanation for _SC_CLK_TCK in readme
HZ=100

t1=$(awk '{sum=$14+$15; print sum}' /proc/${PID}/stat)
u1=$(awk '{print $1}' /proc/uptime)

while true; do
  sleep ${SLEEP_SEC}

  t2=$(awk '{sum=$14+$15; print sum}' /proc/${PID}/stat)
  u2=$(awk '{print $1}' /proc/uptime)

  pcpu=$(($t2 - $t1))
  et=$(awk -v u2="${u2}" -v u1="${u1}" 'BEGIN {printf "%.2f", u2 - u1}')
  # scaling per one CPU
  scaleCPU=$(awk -v et="${et}" -v hz="${HZ}" 'BEGIN {printf "%.6f", 100/(hz * et)}')
  # scaling per all CPUs
  scaleCPUs=$(awk -v scaleCPU="${scaleCPU}" -v cpus="${CPUS}" 'BEGIN {printf "%.6f", scaleCPU/cpus}')

  cpu=$(awk -v pcpu="${pcpu}" -v scaleCPU="${scaleCPU}" 'BEGIN {printf "%.1f", pcpu*scaleCPU}')
  echo %CPU=$cpu

  scaled=$(awk -v pcpu="${pcpu}" -v scaleCPUs="${scaleCPUs}" 'BEGIN {printf "%.1f", pcpu*scaleCPUs}')
  echo scaled=$scaled
  
  t1=$t2
  u1=$u2
done