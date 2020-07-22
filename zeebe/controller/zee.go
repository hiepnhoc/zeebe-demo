package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/zeebe-io/zeebe/clients/go/pkg/entities"
	"github.com/zeebe-io/zeebe/clients/go/pkg/worker"
	"github.com/zeebe-io/zeebe/clients/go/pkg/zbc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)
// INIT
//const BrokerAddr = "172.20.0.4:26500"
const BrokerAddr = "dev_zeebe:26500"
var flow=""

// DATABASE INSTANCE
var collection *mongo.Collection

func CreateCollection(c *mongo.Database) {
	collection = c.Collection("losprocess")
}

// MODEL
type Zeebe struct {
	Customer string `json:"customer"`
	IDCard string `json:"idcard"`
	Score int32 `json:"score"`
	UW  int32 `json:"UW"`
	DI  int32 `json:"DI"`
	PV  int32 `json:"PV"`
	DC  int32 `json:"DC"`
	GetWorkflowKey int64 `json:"GetWorkflowKey"`
	WorkflowInstanceKey int64 `json:"WorkflowInstanceKey"`
	GetBpmnProcessId string `json:"GetBpmnProcessId"`
	GetVersion int32 `json:"GetBpmnProcessId"`
	CreatedDate time.Time `json:"date"`
	LastModifiedDate time.Time `json:"lastModifiedData"`
	CreateAppLog string `json:"CreateAppLog"`
	CheckScoreLog string `json:"CheckScoreLog"`
	DCLog string `json:"DCLog"`
	PVLog string `json:"PVLog"`
	FVLog string `json:"FVLog"`
	UWLog string `json:"UWLog"`
	DILog string `json:"DILog"`
	RetryLog string `json:"RetryLog"`
	LMSLog string `json:"LMSLog"`
	CorrelationKey string `json:"CorrelationKey"`
	FlowGo string `json:"FlowGo"`
}

// CONTROLLER
var zeebeClient zbc.Client

func GetZeebeClient() (zbc.Client, error) {
	if zeebeClient == nil {
		var err error
		zeebeClient, err = zbc.NewClient(&zbc.ClientConfig{
			GatewayAddress:         BrokerAddr,
			UsePlaintextConnection: true,
		})

		if err != nil {
			return nil, err
		}
	}
	return zeebeClient, nil
}

func DeployWorkflow() {
	go func(){
		//initiate
		zeebeClient,err :=GetZeebeClient()

		if err != nil {
			panic(err)
		}

		ctx := context.Background()
		// deploy workflow
		response, err := zeebeClient.NewDeployWorkflowCommand().AddResourceFile("./bpmn/Generalprocess.bpmn").Send(ctx)
		if err != nil {
			panic(err)
		}

		fmt.Println(response.String())

		//end ghi log DB
		gp_createJobTask(zeebeClient)
	}()
}

func CreateIntance(w http.ResponseWriter, r *http.Request){

	//init
	flow=""

	//get body
	reqBody, _ := ioutil.ReadAll(r.Body)

	var zeeBe Zeebe
	json.Unmarshal(reqBody, &zeeBe)

	//initiate
	zeebeClient,err :=GetZeebeClient()

	if err != nil {
		panic(err)
	}

	// After the workflow is deployed.
	variables := make(map[string]interface{})
	variables["score"] = zeeBe.Score
	variables["isUW"] = zeeBe.UW
	variables["isDC"] = zeeBe.DC
	variables["isDI"] = zeeBe.DI
	variables["isPV"] = zeeBe.PV

	request, err := zeebeClient.NewCreateInstanceCommand().BPMNProcessId("general-process").LatestVersion().VariablesFromMap(variables)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	msg, err := request.Send(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("create intance: " + msg.String())

	//ghi log DB
	zeeBe.CreatedDate = time.Now()
	zeeBe.WorkflowInstanceKey=msg.GetWorkflowInstanceKey()
	zeeBe.GetWorkflowKey=msg.GetWorkflowKey()
	zeeBe.GetBpmnProcessId=msg.GetBpmnProcessId()
	zeeBe.GetVersion=msg.GetVersion()

	_, errDB := collection.InsertOne(context.TODO(), zeeBe)

	if errDB != nil {
		log.Printf("Error while inserting new todo into db, Reason: %v\n", err)
		return
	}

	json.NewEncoder(w).Encode("Create Intance Success")
}

func GetAllInstance(w http.ResponseWriter, r *http.Request){
	zeebes := []Zeebe{}
	cursor, err := collection.Find(context.TODO(), bson.M{})

	if err != nil {
		log.Printf("Error while getting all zeebes, Reason: %v\n", err)
		json.NewEncoder(w).Encode("ERROR")
		return
	}

	for cursor.Next(context.TODO()) {
		var todo Zeebe
		cursor.Decode(&todo)
		zeebes = append(zeebes, todo)
	}

	json.NewEncoder(w).Encode(zeebes)
}

func GetById(w http.ResponseWriter, r *http.Request){
	vars := mux.Vars(r)
	key := vars["id"]

	zeebe := Zeebe{}
	err := collection.FindOne(context.TODO(), bson.M{"idcard": key}).Decode(&zeebe)
	if err != nil {
		log.Printf("Error while getting a single todo, Reason: %v\n", err)
		return
	}
	json.NewEncoder(w).Encode(zeebe)
}

func DeleteInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	_, err := collection.DeleteOne(context.TODO(), bson.M{"idcard": id})
	if err != nil {
		log.Printf("Error while deleting a single todo, Reason: %v\n", err)
		return
	}
	json.NewEncoder(w).Encode("Delete Success")
}

func PublishMessageTest(w http.ResponseWriter, r *http.Request){
	//get body
	reqBody, _ := ioutil.ReadAll(r.Body)

	var zeeBe Zeebe
	json.Unmarshal(reqBody, &zeeBe)

	zeebeClient,err :=GetZeebeClient()

	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	result1, err1 := zeebeClient.NewPublishMessageCommand().MessageName("payment-receive").CorrelationKey(zeeBe.CorrelationKey).MessageId(zeeBe.CorrelationKey).Send(ctx)
	if err1 != nil {
		panic(err1)
	}
	fmt.Println(result1.String())

	json.NewEncoder(w).Encode("Publish message success")
}

func gp_createJobTask(client zbc.Client) {
	go func() {
		worker := client.NewJobWorker().JobType("create-application").Handler(createApp).Open()
		worker1 := client.NewJobWorker().JobType("check-score").Handler(checkScore).Open()
		worker2 := client.NewJobWorker().JobType("document-check").Handler(documentCheck).Open()
		worker3 := client.NewJobWorker().JobType("phone-verification").Handler(phoneVerify).Open()
		worker4 := client.NewJobWorker().JobType("field-vefirication").Handler(fieldVefiry).Open()
		worker5 := client.NewJobWorker().JobType("underwriting").Handler(underwriting).Open()
		worker6 := client.NewJobWorker().JobType("disburse").Handler(disburse).Open()
		worker7 := client.NewJobWorker().JobType("call-lms").Handler(callLms).Open()
		worker8 := client.NewJobWorker().JobType("retry").Handler(retry).Open()

		defer worker.Close()
		worker.AwaitClose()

		defer worker1.Close()
		worker1.AwaitClose()

		defer worker2.Close()
		worker2.AwaitClose()

		defer worker3.Close()
		worker3.AwaitClose()

		defer worker4.Close()
		worker4.AwaitClose()

		defer worker5.Close()
		worker5.AwaitClose()

		defer worker6.Close()
		worker6.AwaitClose()

		defer worker7.Close()
		worker7.AwaitClose()

		defer worker8.Close()
		worker8.AwaitClose()
	}()
}

// PROCESS
func createApp(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d Failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()

	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}

	if v, found := variables["isRetryPV"]; found {
		variables["isPV"]=1
		log.Printf("retry: %d" , v)
	}

	if v, found := variables["isRetryDC"]; found {
		variables["isDC"]=1
		log.Printf("retry: %d" , v)
	}

	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	flow+="-> create app"

	//update database
	newData := bson.M{
		"$set": bson.M{
			"createapplog": fmt.Sprintf("%d createApp complete", job.GetKey()),
			"flowgo": flow,
			"lastmodifieddate" : time.Now(),
		},
	}

	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)

	if err1 != nil {
		log.Printf("createApp Error, Reason: %v\n", err)
		return
	}
	//end update db

	log.Printf("%d createApp complete", job.GetKey())

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("createApp complete: %s", res.String())


}

func checkScore(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d checkScore failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()

	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}


	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	flow+="-> check score"

	//update database
	newData := bson.M{
		"$set": bson.M{
			"checkscorelog": fmt.Sprintf("%d checkScore complete", job.GetKey()),
			"flowgo": flow,
			"lastmodifieddate" : time.Now(),
		},
	}
	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)

	if err1 != nil {
		log.Printf("checkscorelog Error, Reason: %v\n", err)
		return
	}
	//end update db

	log.Printf("%d checkScore complete", job.GetKey())

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("checkScore complete: %s", res.String())

}

func documentCheck(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d documentCheck Failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()

	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}

	DC := int(variables["isDC"].(float64))

	if DC==2{
		variables["isRetryDC"]=1
	}

	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	log.Printf("%d documentCheck complete", job.GetKey())

	flow+="-> DC"

	//update database
	newData := bson.M{
		"$set": bson.M{
			"dclog": fmt.Sprintf("%d documentCheck complete", job.GetKey()),
			"flowgo": flow,
			"lastmodifieddate" : time.Now(),
		},
	}

	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)

	if err1 != nil {
		log.Printf("documentCheck Error, Reason: %v\n", err)
		return
	}
	//end update db

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("documentCheck complete: %s", res.String())

}

func phoneVerify(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d phoneVerify Failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()

	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}

	PV := int(variables["isPV"].(float64))

	if PV==2{
		variables["isRetryPV"]=1
	}

	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	log.Printf("%d phoneVerify complete", job.GetKey())

	flow+="-> PV"

	//update database
	zeeBee := Zeebe{}
	errDB := collection.FindOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}).Decode(&zeeBee)
	if errDB != nil {
		log.Printf("Error while getting a single todo, Reason: %v\n", err)
		return
	}
	newData := bson.M{
		"$set": bson.M{
			"pvlog": fmt.Sprintf("%d phoneVerify complete", job.GetKey()),
			"flowgo": flow,
			"lastmodifieddate" : time.Now(),
		},
	}

	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)

	if err1 != nil {
		log.Printf("phoneVerify Error, Reason: %v\n", err)
		return
	}
	//end update db

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("phoneVerify complete: %s", res.String())
}

func fieldVefiry(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d fieldVefiry Failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()

	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}


	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	log.Printf("%d fieldVefiry complete", job.GetKey())

	flow+="-> FV"

	//update database
	zeeBee := Zeebe{}
	errDB := collection.FindOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}).Decode(&zeeBee)
	if errDB != nil {
		log.Printf("Error while getting a single todo, Reason: %v\n", err)
		return
	}
	newData := bson.M{
		"$set": bson.M{
			"fvlog": fmt.Sprintf("%d fieldVefiry complete", job.GetKey()),
			"flowgo": flow,
			"lastmodifieddate" : time.Now(),
		},
	}

	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)

	if err1 != nil {
		log.Printf("fieldVefiry Error, Reason: %v\n", err)
		return
	}
	//end update db

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("fieldVefiry complete: %s", res.String())

}

func underwriting(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d underwriting Failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()

	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}


	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	log.Printf("%d underwriting complete", job.GetKey())

	flow+="-> UW"

	//update database
	newData := bson.M{
		"$set": bson.M{
			"uwlog": fmt.Sprintf("%d underwriting complete", job.GetKey()),
			"flowgo": flow,
			"lastmodifieddate" : time.Now(),
		},
	}

	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)

	if err1 != nil {
		log.Printf("underwriting Error, Reason: %v\n", err)
		return
	}
	//end update db

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("underwriting complete: %s", res.String())

}

func disburse(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d disburse Failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()

	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}


	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	log.Printf("%d disburse complete", job.GetKey())
	flow+="-> DI"

	//update database
	newData := bson.M{
		"$set": bson.M{
			"dilog": fmt.Sprintf("%d disburse complete", job.GetKey()),
			"flowgo": flow,
			"lastmodifieddate" : time.Now(),
		},
	}

	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)

	if err1 != nil {
		log.Printf("disburse Error, Reason: %v\n", err)
		return
	}
	//end update db

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("disburse complete: %s", res.String())

}

func callLms(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d callLms Failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()
	//
	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}


	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	log.Printf("%d callLms complete", job.GetKey())

	flow+="-> LMS"

	//update database
	newData := bson.M{
		"$set": bson.M{
			"lmslog": fmt.Sprintf("%d callLms complete", job.GetKey()),
			"flowgo": flow,
			"lastmodifieddate" : time.Now(),
		},
	}

	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)

	if err1 != nil {
		log.Printf("callLms Error, Reason: %v\n", err)
		return
	}
	//end update db

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("callLms complete: %s", res.String())

}

func retry(client worker.JobClient, job entities.Job) {

	var err error

	defer func() {
		if err != nil {
			log.Printf("%d retry Failed to complete job: %v", job.GetKey(), err)
			client.NewFailJobCommand().JobKey(job.GetKey()).Retries(job.Retries - 1).Send(context.Background())
		}
	}()

	//headers, err := job.GetCustomHeadersAsMap()
	//if err != nil {
	//	return
	//}

	variables, err := job.GetVariablesAsMap()
	if err != nil {
		return
	}

	variables["isDI"]=1

	request, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variables)
	if err != nil {
		return
	}

	log.Printf("%d retry complete", job.GetKey())

	flow+="-> retry(DI)"

	//update database
	newData := bson.M{
		"$set": bson.M{
			"retrylog": fmt.Sprintf("%d retry complete", job.GetKey()),
			"flowgo" : flow,
			"lastmodifieddate" : time.Now(),
		},
	}

	_, err1 := collection.UpdateOne(context.TODO(), bson.M{"workflowinstancekey": job.GetWorkflowInstanceKey()}, newData)
	if err1 != nil {
		log.Printf("retry Error, Reason: %v\n", err)
		return
	}
	//end update db

	ctx := context.Background()
	res, err := request.Send(ctx)

	log.Printf("retry complete: %s", res.String())
}
