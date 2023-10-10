package config

import (
	"flag"
	"time"
)

type ConfigType struct {
	SlavePodSelector  *string
	MasterPodSelector *string
	NodeSelector      *string
	FailoverPoolLabel *string
	SlavePodNamespace *string
	LogEveryRun       *bool
	SleepInterval     *time.Duration
}

var Config = &ConfigType{
	SlavePodSelector:  flag.String("slavePodSelector", "", "slave pod labelSelector"),
	MasterPodSelector: flag.String("masterPodSelector", "", "master pods labelSelector"),
	NodeSelector:      flag.String("nodeSelector", "", "watched nodes labelSelector"),
	FailoverPoolLabel: flag.String("failoverPoolLabel", "", "label specifying ready nodes in failover pool"),
	SlavePodNamespace: flag.String("slavePodNamespace", "", "namespace of slave pod"),
	SleepInterval:     flag.Duration("sleepInterval", 5, "interval in seconds between runs"),
	LogEveryRun:       flag.Bool("logEveryRun", false, "show more info about every run"),
}
