package GlobalValues

import "github.com/boltdb/bolt"

var (
	//TOPIC CONSTANTS
	NO_OF_MESSAGES_IN_DB = 10

	//KAFKA PARAMETERS
	KAFKA_IP_PORT = "198.18.134.26:29092"
	KAFKA_PARTITION = 0
	KAFKA_MINBYTES int = 10e3
	KAFKA_MAXBYTES int = 100e6
	LAST_MSG_OFFSET int64

	//NSO API CALL PARAMS
	NSO_URL="http://198.18.134.26:8080/api/running/devices/device/"
	Token="xyz"
	Headers_json = "Accept:application/vnd.yang.datastore+json"
	Headers_xml = "Accept:application/vnd.yang.data+xml"
	NSOUser = "admin"
	NSOPassword = "admin"

	//TRAFFIC GEN
	TRAFFIC_URL = "http://198.18.133.1:8010/virl_client"
	HEADERS_TRAFFIC_JSON = "Accept:application/json"


	//CDG API CALL PARAMS
	CDG_URL = "https://198.18.134.200/crosswork/cdg/api/v1/topologies"
	CDG_APP_URL = "http://198.18.134.200/crosswork/cdg/api/v1/apps"
	CDG_USERNAME = "admin"
	CDG_PASSWORD = "Admin123!"
	CDG_CONTENTTYPE = "application/json"

	SFO = make(map[string]Snmp)
	FREMONT = make(map[string]Snmp)
	MILPITAS = make(map[string]Snmp)

	//BOLT DB
    DB *bolt.DB
	DBerr error
	Buckets = [8]string{"FREMONT_SFO1", "FREMONT_SFO2", "FREMONT_MILPITAS", "SFO_FREMONT1", "SFO_FREMONT2","SFO_MILPITAS","MILPITAS_FREMONT","MILPITAS_SFO"}
	Topics = [3]string{"FREMONT_IFXTABLE","SFO_IFXTABLE", "MILPITAS_IFXTABLE"}
	Maps = [3]string{"FREMONT", "SFO", "MILPITAS"}
    PopulateOIDs = true

	//Templates
	TemplateHome, SNMPTemplate, MDTTemplate, SendSNMPTemplate, SendMDTTemplate, VIRLTemplate string

	//Cadence Variables
	Router1, Router2, Router3 CadenceData
	FremontID = "016ec6ce-7f77-4096-8640-dfec05cfb9"
	SFOID  = "026ec6ce-7f77-4096-8640-dfec05cfb9"
	MilpitasID = "0383e9e3-5d05-4e2a-8dec-14ecafb757"

	//Kafka Client CLI
	KafkaGetOffset ="/home/cisco/AppCode/kafka_2.12-2.2.0/bin/kafka-run-class.sh"
	KafkaGetOffsetParam = " kafka.tools.GetOffsetShell --broker-list 198.18.134.26:29092 --topic "

)
func InitValues () {
	FREMONT["SFO1"] = Snmp{"0/0/0/1", "1.3.6.1.2.1.31.1.1.1.1.5","1.3.6.1.2.1.31.1.1.1.6.5", "1.3.6.1.2.1.31.1.1.1.10.5", "Fremont",0,0,LOSS_PERCENT_LOWBW}
	FREMONT["SFO2"] = Snmp{"0/0/0/2", "1.3.6.1.2.1.31.1.1.1.1.6","1.3.6.1.2.1.31.1.1.1.7.6", "1.3.6.1.2.1.31.1.1.1.11.6","Fremont",1,0, LOSS_PERCENT_LOWBW}
	FREMONT["MILPITAS"] = Snmp{"0/0/0/3", "1.3.6.1.2.1.31.1.1.1.1.7","1.3.6.1.2.1.31.1.1.1.7.7", "1.3.6.1.2.1.31.1.1.1.10.7", "Fremont", 2,0, LOSS_PERCENT}

    SFO["FREMONT1"] = Snmp{"0/0/0/1", "","", "", "SFO",3,1, LOSS_PERCENT_LOWBW}
	SFO["FREMONT2"] = Snmp{"0/0/0/2" ,"","", "", "SFO",4,1, LOSS_PERCENT_LOWBW}
	SFO["MILPITAS"] = Snmp{"0/0/0/3" ,"","","", "SFO",5,1, LOSS_PERCENT}

	MILPITAS["FREMONT"] = Snmp{"0/0/0/1" ,"","","", "Milpitas",6,2, LOSS_PERCENT}
	MILPITAS["SFO"] = Snmp{"0/0/0/3" ,"","","", "Milpitas",7,2,LOSS_PERCENT}


}

