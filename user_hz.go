package main

/*
#include <unistd.h>
*/
import "C"

func GetUserHZ() int64 {
	return int64(C.sysconf(C._SC_CLK_TCK))
}
