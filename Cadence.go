package GlobalValues

var (
	RouterMetaData [3]CadenceData

	//SNMP CONSTANTS
	POLLING_INTERVAL int64 = 60 //in seconds. Changing this value, requires Deletion of Topology in CDG (if ENVSETUP option is already done to setup CDG), Creation of Topo in CDG.
	SNMP_MESSAGES int64 = 6   //Number of SNMP messages we analyze - to compare (the bytes recvd in the current time interval) to (bytes recvd in other time intervals).  Suggested values - 5,6,7
	OFFSET_FROM_LAST_MSG  int64= 6  // To Save Time, this metric is there to get a trendline how far to go back from the current offset. Reducing this value below the value of "SNMP_MESSAGES" causes time delay at the beginning.
	// Please set this to "0" ONLY if you have old messages in Kafka Topic and there is a significant time lapse between the times when CDG is configured to send metrics to Kafka.

	LOSS_PERCENT int64   //Value of 30 implies 30% loss threshold. Loss greater than this threshold will trigger change automation.
	LOSS_PERCENT_LOWBW int64 = 70
	Add_Debugs = true
)

func InitCadenceData () {
	RouterMetaData[0] = CadenceData{"Fremont", "198.18.1.11", "cisco", "cisco", "SNMP", "", "FREMONT_IFXTABLE", []string{"0/0/0/1", "0/0/0/2", "0/0/0/3"},  POLLING_INTERVAL,  FremontID}
	RouterMetaData[1] = CadenceData{"SFO", "198.18.1.12", "cisco", "cisco", "SNMP","", "SFO_IFXTABLE", []string{"0/0/0/1", "0/0/0/2", "0/0/0/3"}, POLLING_INTERVAL, SFOID}
}



