package main
import (
	"github.com/segmentio/kafka-go"
 	"time"
	"log"
	"fmt"
	"github.com/boltdb/bolt"
   "encoding/json"
	"context"
	"./GlobalValues"
	"regexp"
	"net/http"
	"strings"
	"io/ioutil"
	"strconv"
	"os"
	"os/exec"
	"text/template"
	"bytes"
	"crypto/tls"
	"math/rand"
)


func setupDB () {
	fmt.Println("SETUP BOLT DB")
	GlobalValues.DB, GlobalValues.DBerr = bolt.Open("my.db", 0600, &bolt.Options{Timeout: 1 * time.Second})
	if GlobalValues.DBerr != nil {
		log.Fatal(GlobalValues.DBerr)
	}
}

func createBuckets () {
	for iter:=0; iter <= len(GlobalValues.Buckets)-1; iter ++ {
		fmt.Println("BOLT DB: CREATING BUCKET:", GlobalValues.Buckets[iter])
		if err:= GlobalValues.DB.Update(func(tx *bolt.Tx) error {

			_, err := tx.CreateBucketIfNotExists([]byte(GlobalValues.Buckets[iter]))
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			fmt.Println(" Fatal Error while creating bucket in BoltDB", err)
			log.Fatal(err)
		}
	}
}

func insertData (bucketName string, data GlobalValues.BoltData) {
	//fmt.Printf("Values of bucketName, data are %s %+v", bucketName, data)
	if err := GlobalValues.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		Value, _ := json.Marshal(data)
		offset := strconv.FormatInt(data.Offset,10)
		//fmt.Println("Inside insertData fn" , offset)
		err := b.Put([]byte(offset), Value)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	} /*
	repo := GlobalValues.DB_storm.From(bucketName)
	err := repo.Save(&data)
	if err != nil {
		fmt.Println("Error:",err)
		log.Fatal("Error while inserting data")
	}*/


}

func retrieveData (bucketName string, dataOffset int64) GlobalValues.BoltData {
	var data GlobalValues.BoltData
	GlobalValues.DB.View(func(tx *bolt.Tx) error  {
		b := tx.Bucket([]byte(bucketName))
		offset := strconv.FormatInt(dataOffset,10)
		fmt.Println("Inside retrieveData", offset)
		err := json.Unmarshal(b.Get([]byte(offset)), &data)
		if err != nil {
			log.Fatal("Unable to find data")
		}
		fmt.Println("Read from BoltDB ", data.Offset, data.InOctets, data.OutOctets)
		return nil

    })
	return data
}
func extractSnmpValues (matchOid, Value string) int64 {
	matchStr := `(?m)`+matchOid+`(\s)+=(\s)+([0-9]*)`
	match := regexp.MustCompile(matchStr)
	retVal, err :=  strconv.ParseInt(match.FindStringSubmatch(Value)[3], 10, 64)
	if GlobalValues.Add_Debugs == true {
		//fmt.Println("matchOid and Value", matchOid, retVal)
	}
	if err != nil {
		fmt.Print("Unable to extract the OID value", matchOid)
	}
	return retVal
}

func buildNSOPayload (cmd, ifName string) string {
	var payload string
	if strings.Compare("SHUT",cmd) == 0 {
		payload = "<GigabitEthernet><id>" + ifName + "</id><shutdown/></GigabitEthernet>"
	} else if strings.Compare("NOSHUT",cmd) == 0 {
		payload = "<GigabitEthernet><id>"+ifName+"</id>"+"<shutdown xmlns:nc=\"urn:ietf:params:xml:ns:netconf:base:1.0\"nc:operation=\"delete\"/>"+"</GigabitEthernet>"
	}
	//CLEAN PAYLOAD
	rex := regexp.MustCompile("\r?\n")
	payload = rex.ReplaceAllString(payload, "")

	//CLEAN WHITESPACES IN PAYLOAD
	rexsp := regexp.MustCompile(`>\s+`)
	payload = rexsp.ReplaceAllString(payload, ">")
	return payload
}
func Watcher(seekVal int64, linkName string, linkData GlobalValues.Snmp) {
		var Octets []int64
		if err := GlobalValues.DB.View(func(tx *bolt.Tx) error {
			// Assume bucket exists and has keys
			b := tx.Bucket([]byte(GlobalValues.Buckets[linkData.BucketId]))
			c := b.Cursor()
			count := GlobalValues.SNMP_MESSAGES
			temp:= seekVal
			offset := strconv.FormatInt(seekVal,10)
			for k, v := c.Seek([]byte(offset)) ; k != nil && count > 0; k, v = c.Seek([]byte(strconv.FormatInt(temp,10))) {
				var dataRecord GlobalValues.BoltData
				err := json.Unmarshal(v, &dataRecord )
				if err != nil {
					break
				}
				//Add InOctets
				if linkData.RouterName == "Fremont" {
					Octets = append(Octets, dataRecord.InOctets)
				} else if linkData.RouterName == "SFO" {
					Octets = append(Octets, dataRecord.OutOctets)
				} else if linkData.RouterName == "Milpitas" {
					Octets = append(Octets, dataRecord.InOctets)
				}
				count --
				temp--
				if GlobalValues.Add_Debugs {
					fmt.Printf("Watcher on the Link - %s key=%s, value=%+v\n", linkName, k, dataRecord)
				}
			}
			return nil
		}); err != nil {
			log.Fatal("Fatal Error in %s Watcher",linkName, err)
		}
		fmt.Printf("Watcher: Printing Octets %d gathered on link %s \n", Octets,linkName)
		if findLoss(Octets,linkData) == true && seekVal > GlobalValues.LAST_MSG_OFFSET {
			//var answer string
			fmt.Println("*********************************************")
			fmt.Println("LOSS DETECTED ON THE LINK", linkName)
			fmt.Println("Shutting the Link")
			pushCfgNSO("SHUT", linkData.RouterName,linkData.IfName)
			fmt.Printf("Link %s on Router %s has been shut", linkData.IfName, linkData.RouterName)
			fmt.Println("*********************************************")
		}
}
func findLoss (Octets []int64,linkData GlobalValues.Snmp) bool {
	var retVal bool = false
	var totalDiff, totalDiffAverage int64 = 0, 0
	newDiff := Octets[0] - Octets[1]
	for i := 2; i <= len (Octets) -1 ; i++ {
		totalDiff += Octets[i-1] - Octets[i]
	}
	//GlobalValues.SNMP_LOSS_PERCENTAGE
	totalDiffAverage = totalDiff/(int64 (len(Octets)-2))
	comparedVal := (((100-linkData.LossPercent)* totalDiffAverage)/100)
	fmt.Println("Actual and Compared Values are", newDiff,comparedVal)
	if (newDiff <= comparedVal) {
		retVal = true
	}
	return retVal
}
func extractOID (intfName string, temp GlobalValues.Snmp, text string) GlobalValues.Snmp {
	r, _ := regexp.Compile("(?m)ifDescr (.*) = GigabitEthernet"+intfName)
	ifDesc := r.FindStringSubmatch(text)
	if GlobalValues.Add_Debugs == true {
		fmt.Println("Value of ifDesc", ifDesc[1])
	}

	r, _ = regexp.Compile(`(?m)([0-9]*)$`)
	ifNumber := r.FindStringSubmatch(ifDesc[1])
	if GlobalValues.Add_Debugs == true {
		fmt.Println("Value of ifNumber", ifNumber[0])
	}

	r, _ = regexp.Compile("(?m)ifInOctets (([.0-9]*)" + ifNumber[0] + ") = [0-9]*$")
	inOctetsOid := r.FindStringSubmatch(text)
	if GlobalValues.Add_Debugs == true {
		fmt.Println("Value of inOctetsOid", inOctetsOid[1])
	}
	r, _ = regexp.Compile("(?m)ifOutOctets (([.0-9]*)" + ifNumber[0] + ") = [0-9]*$")
	outOctetsOid := r.FindStringSubmatch(text)
	if GlobalValues.Add_Debugs == true {
		fmt.Println("Value of outOctetsOid", outOctetsOid[1])
	}
	temp.IfOid = ifDesc[1]
	temp.IfInOctetsOid = inOctetsOid[1]
	temp.IfOutOctetsOid = outOctetsOid[1]
	return temp
}
func Listener(linkName string, linkData GlobalValues.Snmp, offset int64, populateOIDs bool) {

	//Define the ReaderConfig Object to access the Router Fremont Topic
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{GlobalValues.KAFKA_IP_PORT},
		Topic:     GlobalValues.Topics[linkData.TopicId],
		Partition: 0,
		MinBytes:  GlobalValues.KAFKA_MINBYTES, // 10KB
		MaxBytes:  GlobalValues.KAFKA_MAXBYTES, // 10MB
	})

	//Get the Message id
	//Lastmessage, _ := r.FetchMessage(context.Background())
	//fmt.Println("Last Message Offset",Lastmessage.Offset)

	if offset <=0 {
		offset = r.Offset()
	}

	//Find out the latest message offset on the Kafka Topic
	//GlobalValues.LAST_MSG_OFFSET, _ = r.ReadLag(context.Background())
	//GlobalValues.LAST_MSG_OFFSET = r.Offset()  --very close
	fmt.Printf("Kafka Topic - %s is at Offset # %d \n",GlobalValues.Topics[linkData.TopicId], offset)
	count := GlobalValues.SNMP_MESSAGES
	
	//Move the offset to where the latest metric is from the Router
	offset = offset-GlobalValues.OFFSET_FROM_LAST_MSG
	if offset <=0 {
		offset = 0
	}
	r.SetOffset(offset)
	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			break
		}

		//fmt.Println("Value of ifOid", linkData.IfOid)
		//matchStr := `(?m)ifDescr (.*) = GigabitEthernet0/0/0/1`
		//ifDesc := regexp.MustCompile(matchStr).FindAllStringSubmatch(string(m.Value),-1)
		//Analyze the Link between Fremont and SFO
		SnmpData := GlobalValues.SnmpData{}
		SnmpData.IfInOctets = extractSnmpValues(linkData.IfInOctetsOid, string(m.Value))
		//fmt.Printf("Value of TotalInOctets for the Link - %s is %d \n",linkName, SnmpData.IfInOctets)
		SnmpData.IfOutOctets = extractSnmpValues(linkData.IfOutOctetsOid, string(m.Value))
		//fmt.Printf("Value of TotalOutOctets for the Link - %s is %d \n",linkName, SnmpData.IfOutOctets)
		dataRecord := GlobalValues.BoltData{m.Offset, SnmpData.IfInOctets, SnmpData.IfOutOctets}

		insertData(GlobalValues.Buckets[linkData.BucketId], dataRecord)
		count --
		if count < 0 {
			//Calculates the metric difference and makes a call to NSO Change Automation
			Watcher(m.Offset, linkName, linkData)
		}

	}
	r.Close()

}
func printSNMP (data GlobalValues.CadenceData) {

	t , _ := template.ParseFiles(GlobalValues.SNMPTemplate)
	err := t.Execute(os.Stdout,data)
	if err != nil {
		panic(err)
	}
}
func printMDT (data GlobalValues.CadenceData) {

	t , _ := template.ParseFiles(GlobalValues.MDTTemplate)
	err := t.Execute(os.Stdout,data)
	if err != nil {
		panic(err)
	}

}

func sendSNMP (data GlobalValues.CadenceData) {
	var tempPayload = new(bytes.Buffer)


	t, _ := template.ParseFiles(GlobalValues.SendSNMPTemplate)
	err := t.Execute(tempPayload,data)
	if err != nil {
		panic(err)
	}
	retCode, retPayload := pushCfgCDG("POST",GlobalValues.CDG_URL, tempPayload.String())
	if strings.Contains(retCode,"200") || strings.Contains(retCode,"204") {
		fmt.Println("Successfully configured Topology for Router"+data.RouterName+"on CDG", retCode, retPayload)
	} else {
		fmt.Println("Failed to configure the Topology for Router"+data.RouterName+"on CDG", retCode, retPayload)
	}
}
func sendMDT (data GlobalValues.CadenceData) {
	var tempPayload = new(bytes.Buffer)
	t, _ := template.ParseFiles(GlobalValues.SendMDTTemplate)
	err := t.Execute(tempPayload,data)
	if err != nil {
		panic(err)
	}
	retCode, retPayload := pushCfgCDG("POST", GlobalValues.CDG_URL, tempPayload.String())
	if strings.Contains(retCode,"200") || strings.Contains(retCode,"204") {
		fmt.Println("Successfully configured Topology for Router"+data.RouterName+"on CDG", retCode, retPayload)
	} else {
		fmt.Println("Failed to configure the Topology for Router"+data.RouterName+"on CDG", retCode, retPayload)
	}
}
func populateOIDs (topicid int, offset int64){
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{GlobalValues.KAFKA_IP_PORT},
		Topic:     GlobalValues.Topics[topicid],
		Partition: 0,
		MinBytes:  GlobalValues.KAFKA_MINBYTES, // 10KB
		MaxBytes:  GlobalValues.KAFKA_MAXBYTES, // 10MB
	})


	r.SetOffset(offset)
	m, _ := r.ReadMessage(context.Background())

	if topicid == 0 {
		fmt.Println("Assigning OIDs for Fremont")
		GlobalValues.FREMONT["SFO1"] = extractOID(GlobalValues.FREMONT["SFO1"].IfName, GlobalValues.FREMONT["SFO1"], string(m.Value))
		GlobalValues.FREMONT["SFO2"] = extractOID(GlobalValues.FREMONT["SFO2"].IfName, GlobalValues.FREMONT["SFO2"], string(m.Value))
		GlobalValues.FREMONT["MILPITAS"] = extractOID(GlobalValues.FREMONT["MILPITAS"].IfName, GlobalValues.FREMONT["MILPITAS"], string(m.Value))

	} else if topicid == 1 {
		fmt.Println("Assigning OIDs for SFO")
		GlobalValues.SFO["FREMONT1"] = extractOID(GlobalValues.SFO["FREMONT1"].IfName, GlobalValues.SFO["FREMONT1"], string(m.Value))
		if GlobalValues.Add_Debugs == true {
			fmt.Printf("SFO to Fremont1 Val%+v", GlobalValues.SFO["FREMONT1"])
		}
		GlobalValues.SFO["FREMONT2"] = extractOID(GlobalValues.SFO["FREMONT2"].IfName, GlobalValues.SFO["FREMONT2"], string(m.Value))
		if GlobalValues.Add_Debugs == true {
			fmt.Printf("SFO to Fremont2%+v", GlobalValues.SFO["FREMONT2"])
		}
		GlobalValues.SFO["MILPITAS"] = extractOID(GlobalValues.SFO["MILPITAS"].IfName, GlobalValues.SFO["MILPITAS"], string(m.Value))
		if GlobalValues.Add_Debugs == true {
			fmt.Printf("SFO to milpitas%+v", GlobalValues.SFO["MILPITAS"])
		}

	} else if topicid == 2 {
		fmt.Println("Assigning OIDs for Milpitas")
		GlobalValues.MILPITAS["SFO"] = extractOID(GlobalValues.MILPITAS["SFO"].IfName, GlobalValues.MILPITAS["SFO"], string(m.Value))
		if GlobalValues.Add_Debugs == true {
			fmt.Printf("Milpitas to SFO Val%+v", GlobalValues.MILPITAS["SFO"])
		}
		GlobalValues.MILPITAS["FREMONT"] = extractOID(GlobalValues.MILPITAS["FREMONT"].IfName, GlobalValues.MILPITAS["FREMONT"], string(m.Value))
		if GlobalValues.Add_Debugs == true {
			fmt.Printf("Milpitas to Fremont %+v", GlobalValues.MILPITAS["FREMONT"])
		}
	}

}
func startWatching () {
	//Initialize Values
	GlobalValues.InitValues()
	//Setup DB
	setupDB()
	//Create Buckets (using Storm)
	createBuckets()

	//Fremont
	out, err := exec.Command(GlobalValues.KafkaGetOffset,"kafka.tools.GetOffsetShell" ,"--broker-list", GlobalValues.KAFKA_IP_PORT, "--topic", GlobalValues.Topics[0]).Output()
	if err != nil {
		fmt.Println("Error", err)
		//log.Fatal(err)
	}
	Offset := regexp.MustCompile(`(?m)([0-9]*)$`).FindString(string(out))
	OffsetVal, err := strconv.ParseInt(Offset, 10, 64)
	//fmt.Println("Value of out, Offset",string(out), Offset)
	fmt.Println("FREMONT beginning Offset",OffsetVal)


	if GlobalValues.PopulateOIDs == true {
		populateOIDs(0,OffsetVal-1)
	}
	for k, v := range GlobalValues.FREMONT {
		k := v.RouterName+"TO"+k
		go Listener(k, v, OffsetVal-1, false)
	}
//************************************************************************************************************************************************************************************************************************************
	//SFO
	out, err = exec.Command(GlobalValues.KafkaGetOffset,"kafka.tools.GetOffsetShell" ,"--broker-list", GlobalValues.KAFKA_IP_PORT, "--topic", GlobalValues.Topics[1]).Output()
	if err != nil {
		fmt.Println("Error", err)
		//log.Fatal(err)
	}
	Offset = regexp.MustCompile(`(?m)([0-9]*)$`).FindString(string(out))
	OffsetVal, err = strconv.ParseInt(Offset, 10, 64)
	fmt.Println("SFO beginning Offset",OffsetVal)

	// Start a Consumer for SNMP Monitoring on SFO
	if GlobalValues.PopulateOIDs == true {
		populateOIDs(1,OffsetVal-1)
	}
	for k, v := range GlobalValues.SFO {
		k := v.RouterName+"TO"+k
		go Listener(k, v, OffsetVal, false)
	}
//************************************************************************************************************************************************************************************************************************************

	//Milpitas
	out, err = exec.Command(GlobalValues.KafkaGetOffset,"kafka.tools.GetOffsetShell" ,"--broker-list", GlobalValues.KAFKA_IP_PORT, "--topic", GlobalValues.Topics[2]).Output()
	if err != nil {
		fmt.Println("Error", err)
		//log.Fatal(err)
	}
	Offset = regexp.MustCompile(`(?m)([0-9]*)$`).FindString(string(out))
	OffsetVal, err = strconv.ParseInt(Offset, 10, 64)
	//fmt.Println("Value of out, Offset",string(out), Offset)
	fmt.Println("Milpitas beginning Offset",OffsetVal)


	if GlobalValues.PopulateOIDs == true {
		populateOIDs(2,OffsetVal-1)
	}
	for k, v := range GlobalValues.MILPITAS {
		k := v.RouterName+"TO"+k
		go Listener(k, v, OffsetVal-1, false)
	}
	//Call Traffic Gen to Trigger Loss
	for {
		fmt.Scanln()
	}
}

func pushCfgNSO(cmd, routerName, ifName string) string{

	//Prepping the Payload
	payload := buildNSOPayload(cmd, ifName)

	//Prepping the Headers
	rexsp := regexp.MustCompile(`\/`)
	ifNameHdr := rexsp.ReplaceAllString(ifName, "%2f")
	url := GlobalValues.NSO_URL + "Router"+routerName + "/config/cisco-ios-xr:interface/GigabitEthernet/"+ifNameHdr
	req, _ := http.NewRequest("PATCH", url, strings.NewReader(payload))
	req.Header.Add("Accept", "application/vnd.yang.data+xml")
	req.Header.Add("Content-Type", "application/vnd.yang.data+xml")
	req.SetBasicAuth(GlobalValues.NSOUser, GlobalValues.NSOPassword)

	//Pushing Payload to NSO
	if GlobalValues.Add_Debugs == true {
		fmt.Println("Pushing the payload to NSO - URL", url)
		fmt.Println("Pushing the payload to NSO - Payload", payload)
	}
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()
	_, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
	//fmt.Println("Response for PushCfg to NSO", string(body))
	fmt.Printf("HTTP status code for %s %s is %s \n" ,routerName, ifName, string(res.Status))
	return string(res.Status)
}

func pushCfgCDG(requestType, url, payload string) (string,string) {
	req, _ := http.NewRequest(requestType, url, strings.NewReader(payload))
	req.Header.Add("Content-Type", GlobalValues.CDG_CONTENTTYPE)
	req.SetBasicAuth(GlobalValues.CDG_USERNAME, GlobalValues.CDG_PASSWORD)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error while calling CDG API: %v\n")
		panic(err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("Error while reading Response from CDG API call: %v\n", err)
	}
	return res.Status,string(body)
}
func pushCfgVIRL(payload string) {
	//Prepping the Headers
	url := GlobalValues.TRAFFIC_URL
	req, _ := http.NewRequest("POST", url, strings.NewReader(payload))
	req.Header.Add("Accept", "application/json")

	//Pushing Payload to NSO
	if GlobalValues.Add_Debugs == true {
		fmt.Println("Pushing the payload to VIRL - URL", url)
		fmt.Println("Pushing the payload to VIRL - Payload", payload)
	}
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Println("Successfully changed the Traffic on VIRL", string(body))
	}
}

func pushtoNSOConfigure (url, payload, routerName string) {
	req, _ := http.NewRequest("PATCH", url, strings.NewReader(payload))
	req.Header.Add("Accept", "application/vnd.yang.data+xml")
	req.Header.Add("Content-Type", "application/vnd.yang.data+xml")
	req.SetBasicAuth(GlobalValues.NSOUser, GlobalValues.NSOPassword)

	//Pushing Payload to NSO
	if GlobalValues.Add_Debugs == true {
		fmt.Println("Pushing the payload to NSO - URL", url)
		fmt.Println("Pushing the payload to NSO - Payload", payload)
	}
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()
	_, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
	//fmt.Println("Response for PushCfg to NSO", string(body))
	fmt.Printf("HTTP status code for %s is %s \n" , routerName, string(res.Status))


}
func main() {

	//Initialize Cadence File
	GlobalValues.InitCadenceData()
	//Initialize Templates
	GlobalValues.TemplateHome = "/var/opt/templates/"
	GlobalValues.SNMPTemplate = GlobalValues.TemplateHome+"PrintSNMP-Template"
	GlobalValues.MDTTemplate = GlobalValues.TemplateHome+"PrintMDT-Template"
	GlobalValues.SendSNMPTemplate = GlobalValues.TemplateHome+"SendSNMP-Template"
	GlobalValues.SendMDTTemplate = GlobalValues.TemplateHome+"SendMDT-Template"
	GlobalValues.VIRLTemplate = GlobalValues.TemplateHome+"VIRL-Template"


	arg := os.Args[1]
	if arg == "CONFIGURE" {
		//SNMP profile
		for iter := 0; iter < len(GlobalValues.RouterMetaData); iter++ {

			payload := "<snmp-server><ifindex>persist</ifindex><community><name>public</name><RW/></community></snmp-server>"
			url := GlobalValues.NSO_URL + "Router"+GlobalValues.RouterMetaData[iter].RouterName+ "/config/cisco-ios-xr:snmp-server"
			pushtoNSOConfigure(url, payload, GlobalValues.RouterMetaData[iter].RouterName)

			if GlobalValues.RouterMetaData[iter].MonitoringType == "MDT" {
				payload := "<telemetry><model-driven> <sensor-group> <id>cdg-generic-counters</id> <sensor-path> <name>Cisco-IOS-XR-infra-statsd-oper:infra-statistics/interfaces/interface/latest/generic-counters</name></sensor-path></sensor-group><subscription><id>cdg-subscription1</id><sensor-group-id><name>cdg-generic-counters</name><sample-interval>60000</sample-interval></sensor-group-id></subscription></model-driven></telemetry>"
				url := GlobalValues.NSO_URL + "Router" + GlobalValues.RouterMetaData[iter].RouterName + "/config/cisco-ios-xr:telemetry"
				pushtoNSOConfigure(url, payload, GlobalValues.RouterMetaData[iter].RouterName)
			}
		}
	} else if arg == "ENVSETUP" {
		//Print the new API calls to SNMP/MDT to CDG
		for iter := 0; iter < len(GlobalValues.RouterMetaData); iter++ {
			fmt.Println("\n*************************" + GlobalValues.RouterMetaData[iter].RouterName + "*************************")
			s1 := rand.NewSource(time.Now().UnixNano())
			r1 := rand.New(s1)
			GlobalValues.RouterMetaData[iter].UniqueId = GlobalValues.RouterMetaData[iter].UniqueId+strconv.Itoa(r1.Intn(1000))
			if GlobalValues.RouterMetaData[iter].MonitoringType == "SNMP" {
				printSNMP(GlobalValues.RouterMetaData[iter])
				sendSNMP(GlobalValues.RouterMetaData[iter])
			} else if GlobalValues.RouterMetaData[iter].MonitoringType == "MDT" {
				printMDT(GlobalValues.RouterMetaData[iter])
				sendMDT(GlobalValues.RouterMetaData[iter])
				sendSNMP(GlobalValues.RouterMetaData[iter])
			}

		}
	} else if arg == "TEST" {
		//fmt.Println("Value of GlobalValues",GlobalValues.Test)

	} else if arg == "STARTWATCHER" {
		GlobalValues.LOSS_PERCENT, _ = strconv.ParseInt(os.Args[2],10,64)
		startWatching()
	} else if arg == "DELETE" {
		fmt.Println("This function takes the following arguments - Collection Profile")
		topologyID := os.Args[2]
		retCode, retPayload := pushCfgCDG("DELETE",GlobalValues.CDG_URL+"/"+topologyID,"")
		if strings.Contains(retCode,"200") || strings.Contains(retCode,"204") {
			fmt.Println("Successfully deleted the Collection Profile on CDG", topologyID, retCode, retPayload)
		} else {
			fmt.Println("Failed to delete the Collection Profile on CDG", topologyID, retCode, retPayload)
		}
	} else if arg == "LIST" {
		_, retResponse := pushCfgCDG("GET",GlobalValues.CDG_URL,"")
		//fmt.Println("Response", retResponse)
		var result interface{}
		json.Unmarshal([]byte(retResponse), &result)
		m := result.(map[string]interface{})
		for _, v := range m {
			switch vv := v.(type) {
			case []interface{}:
				//fmt.Println(k, "is an array:")
				for _, u := range vv {
					str := fmt.Sprintf("%v", u)
					statusVal := regexp.MustCompile(`(?m)status:[^\s]*`).FindString(str)
					topologyIdVal := regexp.MustCompile(`(?m)topologyId:[^\s]*`).FindString(str)
					destinationTopicVal := regexp.MustCompile(`(?m)destinationTopic:[^\s]*`).FindString(str)
					appTopologyRefVal := regexp.MustCompile(`(?m)appTopologyRef:[^\s]*`).FindString(str)
					if strings.Contains(statusVal,"ACTIVE") {
						fmt.Println("Values--->", topologyIdVal, statusVal, appTopologyRefVal, destinationTopicVal)
					}
				}
			default:
				//fmt.Println(k, "is of a type I don't know how to handle")
			}
		}
	} else if arg == "UNSHUTINTF" {
		for iter := 0; iter < len(GlobalValues.RouterMetaData); iter++ {
			for iter1 := 0 ; iter1 < len(GlobalValues.RouterMetaData[iter].InterfaceNames); iter1++ {
				pushCfgNSO("NOSHUT", GlobalValues.RouterMetaData[iter].RouterName,GlobalValues.RouterMetaData[iter].InterfaceNames[iter1] )
			}
		}
	} else if arg == "TRAFFIC" {
		fmt.Println("This function takes the following arguments - SRC, DEST, DELAY, LINK LOSS")
		LinkDelay := GlobalValues.TrafficData{os.Args[2],os.Args[3],os.Args[4],os.Args[5]}
		var tempPayload = new(bytes.Buffer)
		t, _ := template.ParseFiles(GlobalValues.VIRLTemplate)
		err := t.Execute(tempPayload,LinkDelay)
		if err != nil {
			fmt.Println("Unable to change the VIRL Traffic")
			panic(err)
		}
		pushCfgVIRL(tempPayload.String())
	}
}




