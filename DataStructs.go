package GlobalValues

type Snmp struct {
	IfName string
	IfOid string
	IfInOctetsOid string
	IfOutOctetsOid string
	RouterName string
	BucketId int
	TopicId int
	LossPercent int64
}

type SnmpData struct {
	IfInOctets int64
	IfOutOctets int64
}

type BoltData struct {
	Offset int64 `storm:"id"`// primary key
	InOctets int64 //not indexed
	OutOctets int64 //not indexed
}

type CadenceData struct {
	RouterName string
	RouterIP string
	RouterUsername string
	RouterPassword string
	MonitoringType string
	SensorPath string
	KafkaTopic string
	InterfaceNames []string
	SNMPPollingInterval int64
	UniqueId string
}

type TrafficData struct {
	Src string
	Dst string
	Delay string
	Loss string
}