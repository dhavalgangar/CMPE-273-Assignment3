package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"httprouter"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//DbName and DbCollection constants
const (
	DbName       = "mongo_db"
	DbCollection = "location"
)

var nextDestLocID = 0
var t = 0

//Counter for sequence
type Counter struct {
	ID       string `bson:"_id"`
	Sequence int    `bson:"seq"`
}

//RespUber to get response from /v1/requests
type RespUber struct {
	Driver          interface{} `json:"driver"`
	Eta             int         `json:"eta"`
	Location        interface{} `json:"location"`
	RequestID       string      `json:"request_id"`
	Status          string      `json:"status"`
	SurgeMultiplier interface{} `json:"surge_multiplier"`
	Vehicle         interface{} `json:"vehicle"`
}

//UberResponse1 for getting price estimates
type UberResponse1 struct {
	Prices []struct {
		CurrencyCode         string  `json:"currency_code"`
		DisplayName          string  `json:"display_name"`
		Distance             float64 `json:"distance"`
		Duration             int     `json:"duration"`
		Estimate             string  `json:"estimate"`
		HighEstimate         int     `json:"high_estimate"`
		LocalizedDisplayName string  `json:"localized_display_name"`
		LowEstimate          int     `json:"low_estimate"`
		Minimum              int     `json:"minimum"`
		ProductID            string  `json:"product_id"`
		SurgeMultiplier      int     `json:"surge_multiplier"`
	} `json:"prices"`
}

//Req1 to add locations
type Req1 struct {
	LocationIds            []string `json:"location_ids"`
	StartingFromLocationID string   `json:"starting_from_location_id"`
}

//Resp1 to give planning
type Resp1 struct {
	ID                     int     `bson:"_id"`
	StartingFromLocationID int     `bson:"starting_from_location_id"`
	BestRouteLocationIds   []int   `bson:"best_route_location_ids"`
	Status                 string  `bson:"status"`
	TotalDistance          float64 `bson:"total_distance"`
	TotalUberCosts         int     `bson:"total_uber_costs"`
	TotalUberDuration      int     `bson:"total_uber_duration"`
}

//Resp2 to give modified planning
type Resp2 struct {
	ID                        int     `bson:"_id"`
	StartingFromLocationID    int     `bson:"starting_from_location_id"`
	NextDestinationLocationID int     `bson:"next_destination_location_id"`
	BestRouteLocationIds      []int   `bson:"best_route_location_ids"`
	Status                    string  `bson:"status"`
	TotalDistance             float64 `bson:"total_distance"`
	TotalUberCosts            int     `bson:"total_uber_costs"`
	TotalUberDuration         int     `bson:"total_uber_duration"`
	UberWaitTimeEta           int     `bson:"uber_wait_time_eta"`
}

//ResponseDB for getting response from DB
type ResponseDB struct {
	ID         int    `bson:"_id"`
	Name       string `bson:"name"`
	Address    string `bson:"address"`
	City       string `bson:"city"`
	State      string `bson:"state"`
	Zip        string `bson:"zip"`
	Coordinate struct {
		Lat float64 `bson:"lat"`
		Lng float64 `bson:"lng"`
	} `bson:"coordinate"`
}

//PlanTrip function to plan a trip
func PlanTrip(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	//accept request from user and decode it to a stuct
	request1 := Req1{}

	json.NewDecoder(r.Body).Decode(&request1)

	//establish mongodb connection
	session, err := mgo.Dial("mongodb://dhaval:dhaval123@ds041144.mongolab.com:41144/mongo_db")
	checkError(err)
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)
	c := session.DB(DbName).C(DbCollection)

	responseDB1 := ResponseDB{}
	responseDB2 := ResponseDB{}

	locationArray := make([]int, len(request1.LocationIds))
	priceEstimate := make([]int, len(request1.LocationIds))
	var rsponseLocationSlice = []int{}

	for i := 0; i < len(request1.LocationIds); i++ {
		locationArray[i], _ = strconv.Atoi(request1.LocationIds[i])
	}

	startLocation, _ := strconv.Atoi(request1.StartingFromLocationID)

	for i := 0; i < len(request1.LocationIds); i++ {
		fmt.Print("-----------Outer loop starts, start location - ")
		fmt.Println(startLocation)
		err = c.FindId(startLocation).One(&responseDB1)
		startLatitude := responseDB1.Coordinate.Lat
		startLongitude := responseDB1.Coordinate.Lng
		for j := 0; j < len(request1.LocationIds); j++ {
			fmt.Print("-----------inner loop starts, start location - ")
			fmt.Println(startLocation)
			fmt.Print("resp loc slice = ")
			fmt.Print(rsponseLocationSlice)
			fmt.Print("location array = ")
			fmt.Println(locationArray[j])
			if !contains(locationArray[j], rsponseLocationSlice) {
				err = c.FindId(locationArray[j]).One(&responseDB2)
				endLatitude := responseDB2.Coordinate.Lat
				endLongitude := responseDB2.Coordinate.Lng
				priceEstimate[j] = GetPriceEstimate(startLatitude, startLongitude, endLatitude, endLongitude)
				fmt.Print("price Estimate - ")
				fmt.Println(priceEstimate)
			}
			fmt.Println("---------inner loop ends-----------")
		}
		fmt.Println("---------outer loop ends-----------")
		minIndex := getMinIndex(priceEstimate)
		fmt.Print("price estimate - ")
		fmt.Println(priceEstimate)
		fmt.Print("min Index - ")
		fmt.Println(minIndex)
		rsponseLocationSlice = append(rsponseLocationSlice, locationArray[minIndex])
		startLocation = locationArray[minIndex]
		err = c.FindId(startLocation).One(&responseDB1)
		//Initialize the priceEstimate array to 0.
		for i := 0; i < len(priceEstimate); i++ {
			priceEstimate[i] = 0
		}
	}
	startLocation, _ = strconv.Atoi(request1.StartingFromLocationID)
	//compute all the values for response struct
	totalUberCosts, totalUberDuration, totalDistance := computeValues(rsponseLocationSlice, startLocation)

	response1 := Resp1{}
	response1.StartingFromLocationID = startLocation
	response1.Status = "planning"
	response1.TotalDistance = totalDistance
	response1.TotalUberCosts = totalUberCosts
	response1.TotalUberDuration = totalUberDuration
	response1.ID = getNextSequence()
	for i := 0; i < len(rsponseLocationSlice); i++ {
		response1.BestRouteLocationIds = append(response1.BestRouteLocationIds, rsponseLocationSlice[i])
	}
	nextDestLocID = response1.BestRouteLocationIds[0]
	err = c.Insert(&response1)
	checkError(err)

	// Marshal provided interface into JSON structure
	uj, _ := json.Marshal(response1)

	// Write content-type, statuscode, payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	fmt.Fprintf(w, "%s", uj)

}

//getNextSequence function to get next sequnce from DB
func getNextSequence() int {

	//var doc Seq
	session, err := mgo.Dial("mongodb://dhaval:dhaval123@ds041144.mongolab.com:41144/mongo_db")
	checkError(err)
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)
	c := session.DB(DbName).C("counters")

	change := mgo.Change{
		Update:    bson.M{"$inc": bson.M{"seq": 1}},
		ReturnNew: true,
	}

	doc := Counter{}
	_, err = c.Find(bson.M{"_id": "userid"}).Apply(change, &doc)
	checkError(err)
	return doc.Sequence
}

//computeValues function
func computeValues(locationArray []int, startLocation int) (int, int, float64) {
	totalUberCosts := 0
	totalUberDuration := 0
	totalDistance := 0.0
	var tempLocArr = []int{}
	//establish mongodb connection
	session, err := mgo.Dial("mongodb://dhaval:dhaval123@ds041144.mongolab.com:41144/mongo_db")
	checkError(err)
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)
	c := session.DB(DbName).C(DbCollection)

	responseDB1 := ResponseDB{}

	tempLocArr = append(tempLocArr, startLocation)
	for i := 0; i < len(locationArray); i++ {
		tempLocArr = append(tempLocArr, locationArray[i])
	}
	tempLocArr = append(tempLocArr, startLocation)

	fmt.Print("tempLocArr - ")
	fmt.Println(tempLocArr)

	for i := 0; i < len(tempLocArr)-1; i++ {

		err = c.FindId(tempLocArr[i]).One(&responseDB1)
		startLatitude := responseDB1.Coordinate.Lat
		startLongitude := responseDB1.Coordinate.Lng
		err = c.FindId(tempLocArr[i+1]).One(&responseDB1)
		endLatitude := responseDB1.Coordinate.Lat
		endLongitude := responseDB1.Coordinate.Lng
		fmt.Print("end points - ")
		fmt.Print(tempLocArr[i])
		fmt.Print("  ")
		fmt.Println(tempLocArr[i+1])
		a, b, c := GetAllEstimates(startLatitude, startLongitude, endLatitude, endLongitude)
		totalUberCosts = totalUberCosts + a
		totalUberDuration = totalUberDuration + b
		totalDistance = totalDistance + c
		fmt.Print("Details - ")
		fmt.Println(a)
		fmt.Println(b)
		fmt.Println(c)
	}

	return totalUberCosts, totalUberDuration, totalDistance
}

//getMinIndex func to get min index
func getMinIndex(priceEstimate []int) int {
	min := 999
	minIndex := 0
	for i := 0; i < len(priceEstimate); i++ {
		if priceEstimate[i] < min && priceEstimate[i] > 5 {
			min = priceEstimate[i]
			minIndex = i
		}
	}
	return minIndex
}

//contains func to check if x is already present in arr
func contains(x int, arr []int) bool {

	if len(arr) == 0 {
		return false
	}
	for i := 0; i < len(arr); i++ {
		if x == arr[i] {
			return true
		}
	}
	return false
}

//GetPriceEstimate function to get low price estimate from uberAPI
func GetPriceEstimate(startLat float64, startLng float64, endlat float64, endLng float64) int {

	var buffer bytes.Buffer
	var uberResponse UberResponse1

	buffer.WriteString("https://sandbox-api.uber.com/v1/estimates/price?server_token=jGVdEr6ES63Bs--ZJDpjDEXq1EnbCt3cAMYhqSDz&start_latitude=")
	buffer.WriteString(strconv.FormatFloat(startLat, 'g', -1, 64))
	buffer.WriteString("&start_longitude=")
	buffer.WriteString(strconv.FormatFloat(startLng, 'g', -1, 64))
	buffer.WriteString("&end_latitude=")
	buffer.WriteString(strconv.FormatFloat(endlat, 'g', -1, 64))
	buffer.WriteString("&end_longitude=")
	buffer.WriteString(strconv.FormatFloat(endLng, 'g', -1, 64))
	urlUberAPI := buffer.String()
	fmt.Println(urlUberAPI)
	response, err := http.Get(urlUberAPI)
	checkError(err)
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	checkError(err)

	json.Unmarshal([]byte(contents), &uberResponse)

	return uberResponse.Prices[0].LowEstimate
}

//GetAllEstimates function to get low price estimate from uberAPI
func GetAllEstimates(startLat float64, startLng float64, endlat float64, endLng float64) (int, int, float64) {

	var buffer bytes.Buffer
	var uberResponse UberResponse1

	buffer.WriteString("https://sandbox-api.uber.com/v1/estimates/price?server_token=jGVdEr6ES63Bs--ZJDpjDEXq1EnbCt3cAMYhqSDz&start_latitude=")
	buffer.WriteString(strconv.FormatFloat(startLat, 'g', -1, 64))
	buffer.WriteString("&start_longitude=")
	buffer.WriteString(strconv.FormatFloat(startLng, 'g', -1, 64))
	buffer.WriteString("&end_latitude=")
	buffer.WriteString(strconv.FormatFloat(endlat, 'g', -1, 64))
	buffer.WriteString("&end_longitude=")
	buffer.WriteString(strconv.FormatFloat(endLng, 'g', -1, 64))
	urlUberAPI := buffer.String()
	fmt.Println(urlUberAPI)
	response, err := http.Get(urlUberAPI)
	checkError(err)
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	checkError(err)

	json.Unmarshal([]byte(contents), &uberResponse)

	return uberResponse.Prices[0].LowEstimate, uberResponse.Prices[0].Duration, uberResponse.Prices[0].Distance
}

//GetTripDetails function to check trip details and status
func GetTripDetails(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	id := p.ByName("tripid")
	oid, _ := strconv.Atoi(id)
	//establish mongodb connection
	session, err := mgo.Dial("mongodb://dhaval:dhaval123@ds041144.mongolab.com:41144/mongo_db")
	checkError(err)
	defer session.Close()
	//insert values in mongo_db
	session.SetMode(mgo.Monotonic, true)
	c := session.DB(DbName).C(DbCollection)

	resultResponse := Resp1{}

	err = c.FindId(oid).One(&resultResponse)
	checkError(err)
	uj, _ := json.Marshal(resultResponse)

	// Write content-type, statuscode, payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	fmt.Fprintf(w, "%s", uj)

}

//RequestACar function to request a car for the next destination
func RequestACar(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	id := p.ByName("tripid")
	oid, _ := strconv.Atoi(id)
	//establish mongodb connection
	session, err := mgo.Dial("mongodb://dhaval:dhaval123@ds041144.mongolab.com:41144/mongo_db")
	checkError(err)
	defer session.Close()
	session.SetMode(mgo.Monotonic, true)
	c := session.DB(DbName).C(DbCollection)

	resultResponse := Resp1{}
	err = c.FindId(oid).One(&resultResponse)
	checkError(err)

	responseDB1 := ResponseDB{}
	resp2 := Resp2{}
	var respUber1 RespUber

	if t == 0 {
		err = c.FindId(resultResponse.StartingFromLocationID).One(&responseDB1)
		startLatitude := strconv.FormatFloat(responseDB1.Coordinate.Lat, 'g', -1, 64)
		startLongitude := strconv.FormatFloat(responseDB1.Coordinate.Lng, 'g', -1, 64)

		err = c.FindId(resultResponse.BestRouteLocationIds[t]).One(&responseDB1)
		endLatitude := strconv.FormatFloat(responseDB1.Coordinate.Lat, 'g', -1, 64)
		endLongitude := strconv.FormatFloat(responseDB1.Coordinate.Lng, 'g', -1, 64)
		var jsonStr = []byte(`{"start_latitude":"` + startLatitude + `","start_longitude":"` + startLongitude + `","end_latitude":"` + endLatitude + `","end_longitude":"` + endLongitude + `","product_id":"a1111c8c-c720-46c3-8534-2fcdd730040d"}`)

		urlreq := "https://sandbox-api.uber.com/v1/requests"
		req, err := http.NewRequest("POST", urlreq, bytes.NewBuffer(jsonStr))
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsImhpc3RvcnkiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiZjQwMDI4ZjEtNmVmMy00N2QxLWFiOTMtNzlkYWNhNWQ2ZmRhIiwiaXNzIjoidWJlci11czEiLCJqdGkiOiI2NWJmYmQ4NS0wNWZjLTQ2MzUtODRjYS00YzIyOGI1YjgyNTMiLCJleHAiOjE0NTA3NTg4MTMsImlhdCI6MTQ0ODE2NjgxMiwidWFjdCI6ImVFcW40WXRLMmc4NHZOcWZCVUNOdkVQc252aE5ROCIsIm5iZiI6MTQ0ODE2NjcyMiwiYXVkIjoiYnRNSUFXbkpEQzdnUF85SzhfQzBxUjI2VWlNQmx5b2UifQ.CBm5HucgCKwOi-MkWdLKaJ9NMMnfCpHo1gjQ17VkYRFgMDanxPeQYjDZUhWUlKsb3wJmYdWs5CYeVbeFcKm_huIC5DmjNJ4QRODx3-XgetKdfHBwAwuWkUypTfMswOlWV1JONmZur9YBTHHqBIqLbcNKKj7HUtvMvQv0w-Dm29XuSoJtIMnQkDzBqyxUGQjkWcv1WjSqJAsFDUo_m8S6bSSWOR7xQFhQCm4gXH_wZ3JKaPv6bqOkfyUfwZ5YqFZzT5ybWhd6ZoLcUiFRsBZPfAqPA0WgHIdrdeS2DKGcrjboBjL4MOjS9yT5BWdT59RrBZQy19650Q1MqRxf1fSW_A")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		checkError(err)

		defer resp.Body.Close()
		contents, errmesg := ioutil.ReadAll(resp.Body)
		checkError(errmesg)

		json.Unmarshal([]byte(contents), &respUber1)

		jsonStr = []byte(`{"status": "accepted"}`)

		var buffer bytes.Buffer
		buffer.WriteString("https://sandbox-api.uber.com/v1/sandbox/requests?")
		buffer.WriteString(respUber1.RequestID)
		urlreq = buffer.String()
		req, err = http.NewRequest("POST", urlreq, bytes.NewBuffer(jsonStr))
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsImhpc3RvcnkiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiZjQwMDI4ZjEtNmVmMy00N2QxLWFiOTMtNzlkYWNhNWQ2ZmRhIiwiaXNzIjoidWJlci11czEiLCJqdGkiOiI2NWJmYmQ4NS0wNWZjLTQ2MzUtODRjYS00YzIyOGI1YjgyNTMiLCJleHAiOjE0NTA3NTg4MTMsImlhdCI6MTQ0ODE2NjgxMiwidWFjdCI6ImVFcW40WXRLMmc4NHZOcWZCVUNOdkVQc252aE5ROCIsIm5iZiI6MTQ0ODE2NjcyMiwiYXVkIjoiYnRNSUFXbkpEQzdnUF85SzhfQzBxUjI2VWlNQmx5b2UifQ.CBm5HucgCKwOi-MkWdLKaJ9NMMnfCpHo1gjQ17VkYRFgMDanxPeQYjDZUhWUlKsb3wJmYdWs5CYeVbeFcKm_huIC5DmjNJ4QRODx3-XgetKdfHBwAwuWkUypTfMswOlWV1JONmZur9YBTHHqBIqLbcNKKj7HUtvMvQv0w-Dm29XuSoJtIMnQkDzBqyxUGQjkWcv1WjSqJAsFDUo_m8S6bSSWOR7xQFhQCm4gXH_wZ3JKaPv6bqOkfyUfwZ5YqFZzT5ybWhd6ZoLcUiFRsBZPfAqPA0WgHIdrdeS2DKGcrjboBjL4MOjS9yT5BWdT59RrBZQy19650Q1MqRxf1fSW_A")
		req.Header.Set("Content-Type", "application/json")

		client = &http.Client{}
		resp, err = client.Do(req)
		checkError(err)

		resp2.ID = resultResponse.ID
		resp2.NextDestinationLocationID = resultResponse.BestRouteLocationIds[t]
		resp2.StartingFromLocationID = resultResponse.StartingFromLocationID
		resp2.TotalDistance = resultResponse.TotalDistance
		resp2.TotalUberCosts = resultResponse.TotalUberCosts
		resp2.TotalUberDuration = resultResponse.TotalUberDuration
		resp2.UberWaitTimeEta = respUber1.Eta

		for i := 0; i < len(resultResponse.BestRouteLocationIds); i++ {
			resp2.BestRouteLocationIds = append(resp2.BestRouteLocationIds, resultResponse.BestRouteLocationIds[i])
		}
		nextDestLocID = resultResponse.BestRouteLocationIds[t+1]
		t = t + 1
		if nextDestLocID == resultResponse.StartingFromLocationID {
			resp2.Status = "completed"
			// Marshal provided interface into JSON structure
			uj, _ := json.Marshal(resp2)

			// Write content-type, statuscode, payload
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			fmt.Fprintf(w, "%s", uj)
			os.Exit(1)
		} else {
			resp2.Status = "requesting"
			// Marshal provided interface into JSON structure
			uj, _ := json.Marshal(resp2)

			// Write content-type, statuscode, payload
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			fmt.Fprintf(w, "%s", uj)
		}
	} else if t < len(resultResponse.BestRouteLocationIds)-1 && t != 0 {

		err = c.FindId(resultResponse.BestRouteLocationIds[t]).One(&responseDB1)
		startLatitude := strconv.FormatFloat(responseDB1.Coordinate.Lat, 'g', -1, 64)
		startLongitude := strconv.FormatFloat(responseDB1.Coordinate.Lng, 'g', -1, 64)

		err = c.FindId(resultResponse.BestRouteLocationIds[t+1]).One(&responseDB1)
		endLatitude := strconv.FormatFloat(responseDB1.Coordinate.Lat, 'g', -1, 64)
		endLongitude := strconv.FormatFloat(responseDB1.Coordinate.Lng, 'g', -1, 64)
		var jsonStr = []byte(`{"start_latitude":"` + startLatitude + `","start_longitude":"` + startLongitude + `","end_latitude":"` + endLatitude + `","end_longitude":"` + endLongitude + `","product_id":"a1111c8c-c720-46c3-8534-2fcdd730040d"}`)

		urlreq := "https://sandbox-api.uber.com/v1/requests"
		req, err := http.NewRequest("POST", urlreq, bytes.NewBuffer(jsonStr))
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsImhpc3RvcnkiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiZjQwMDI4ZjEtNmVmMy00N2QxLWFiOTMtNzlkYWNhNWQ2ZmRhIiwiaXNzIjoidWJlci11czEiLCJqdGkiOiI2NWJmYmQ4NS0wNWZjLTQ2MzUtODRjYS00YzIyOGI1YjgyNTMiLCJleHAiOjE0NTA3NTg4MTMsImlhdCI6MTQ0ODE2NjgxMiwidWFjdCI6ImVFcW40WXRLMmc4NHZOcWZCVUNOdkVQc252aE5ROCIsIm5iZiI6MTQ0ODE2NjcyMiwiYXVkIjoiYnRNSUFXbkpEQzdnUF85SzhfQzBxUjI2VWlNQmx5b2UifQ.CBm5HucgCKwOi-MkWdLKaJ9NMMnfCpHo1gjQ17VkYRFgMDanxPeQYjDZUhWUlKsb3wJmYdWs5CYeVbeFcKm_huIC5DmjNJ4QRODx3-XgetKdfHBwAwuWkUypTfMswOlWV1JONmZur9YBTHHqBIqLbcNKKj7HUtvMvQv0w-Dm29XuSoJtIMnQkDzBqyxUGQjkWcv1WjSqJAsFDUo_m8S6bSSWOR7xQFhQCm4gXH_wZ3JKaPv6bqOkfyUfwZ5YqFZzT5ybWhd6ZoLcUiFRsBZPfAqPA0WgHIdrdeS2DKGcrjboBjL4MOjS9yT5BWdT59RrBZQy19650Q1MqRxf1fSW_A")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		checkError(err)

		defer resp.Body.Close()
		contents, errmesg := ioutil.ReadAll(resp.Body)
		checkError(errmesg)

		json.Unmarshal([]byte(contents), &respUber1)

		jsonStr = []byte(`{"status": "accepted"}`)

		var buffer bytes.Buffer
		buffer.WriteString("https://sandbox-api.uber.com/v1/sandbox/requests?")
		buffer.WriteString(respUber1.RequestID)
		urlreq = buffer.String()
		req, err = http.NewRequest("POST", urlreq, bytes.NewBuffer(jsonStr))
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsImhpc3RvcnkiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiZjQwMDI4ZjEtNmVmMy00N2QxLWFiOTMtNzlkYWNhNWQ2ZmRhIiwiaXNzIjoidWJlci11czEiLCJqdGkiOiI2NWJmYmQ4NS0wNWZjLTQ2MzUtODRjYS00YzIyOGI1YjgyNTMiLCJleHAiOjE0NTA3NTg4MTMsImlhdCI6MTQ0ODE2NjgxMiwidWFjdCI6ImVFcW40WXRLMmc4NHZOcWZCVUNOdkVQc252aE5ROCIsIm5iZiI6MTQ0ODE2NjcyMiwiYXVkIjoiYnRNSUFXbkpEQzdnUF85SzhfQzBxUjI2VWlNQmx5b2UifQ.CBm5HucgCKwOi-MkWdLKaJ9NMMnfCpHo1gjQ17VkYRFgMDanxPeQYjDZUhWUlKsb3wJmYdWs5CYeVbeFcKm_huIC5DmjNJ4QRODx3-XgetKdfHBwAwuWkUypTfMswOlWV1JONmZur9YBTHHqBIqLbcNKKj7HUtvMvQv0w-Dm29XuSoJtIMnQkDzBqyxUGQjkWcv1WjSqJAsFDUo_m8S6bSSWOR7xQFhQCm4gXH_wZ3JKaPv6bqOkfyUfwZ5YqFZzT5ybWhd6ZoLcUiFRsBZPfAqPA0WgHIdrdeS2DKGcrjboBjL4MOjS9yT5BWdT59RrBZQy19650Q1MqRxf1fSW_A")
		req.Header.Set("Content-Type", "application/json")

		client = &http.Client{}
		resp, err = client.Do(req)
		checkError(err)

		resp2.ID = resultResponse.ID
		resp2.NextDestinationLocationID = resultResponse.BestRouteLocationIds[t]
		resp2.StartingFromLocationID = resultResponse.StartingFromLocationID
		resp2.TotalDistance = resultResponse.TotalDistance
		resp2.TotalUberCosts = resultResponse.TotalUberCosts
		resp2.TotalUberDuration = resultResponse.TotalUberDuration
		resp2.UberWaitTimeEta = respUber1.Eta

		for i := 0; i < len(resultResponse.BestRouteLocationIds); i++ {
			resp2.BestRouteLocationIds = append(resp2.BestRouteLocationIds, resultResponse.BestRouteLocationIds[i])
		}
		if t+1 >= len(resultResponse.BestRouteLocationIds) {
			nextDestLocID = resultResponse.BestRouteLocationIds[t]
		} else {
			nextDestLocID = resultResponse.BestRouteLocationIds[t+1]
		}
		t = t + 1
		if nextDestLocID == resultResponse.StartingFromLocationID {
			resp2.Status = "completed"
			// Marshal provided interface into JSON structure
			uj, _ := json.Marshal(resp2)

			// Write content-type, statuscode, payload
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			fmt.Fprintf(w, "%s", uj)

		} else {
			resp2.Status = "requesting"
			// Marshal provided interface into JSON structure
			uj, _ := json.Marshal(resp2)

			// Write content-type, statuscode, payload
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			fmt.Fprintf(w, "%s", uj)
		}

	} else if t >= len(resultResponse.BestRouteLocationIds) {
		err = c.FindId(resultResponse.BestRouteLocationIds[t-1]).One(&responseDB1)
		startLatitude := strconv.FormatFloat(responseDB1.Coordinate.Lat, 'g', -1, 64)
		startLongitude := strconv.FormatFloat(responseDB1.Coordinate.Lng, 'g', -1, 64)

		err = c.FindId(resultResponse.StartingFromLocationID).One(&responseDB1)
		endLatitude := strconv.FormatFloat(responseDB1.Coordinate.Lat, 'g', -1, 64)
		endLongitude := strconv.FormatFloat(responseDB1.Coordinate.Lng, 'g', -1, 64)
		var jsonStr = []byte(`{"start_latitude":"` + startLatitude + `","start_longitude":"` + startLongitude + `","end_latitude":"` + endLatitude + `","end_longitude":"` + endLongitude + `","product_id":"a1111c8c-c720-46c3-8534-2fcdd730040d"}`)

		urlreq := "https://sandbox-api.uber.com/v1/requests"
		req, err := http.NewRequest("POST", urlreq, bytes.NewBuffer(jsonStr))
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsImhpc3RvcnkiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiZjQwMDI4ZjEtNmVmMy00N2QxLWFiOTMtNzlkYWNhNWQ2ZmRhIiwiaXNzIjoidWJlci11czEiLCJqdGkiOiI2NWJmYmQ4NS0wNWZjLTQ2MzUtODRjYS00YzIyOGI1YjgyNTMiLCJleHAiOjE0NTA3NTg4MTMsImlhdCI6MTQ0ODE2NjgxMiwidWFjdCI6ImVFcW40WXRLMmc4NHZOcWZCVUNOdkVQc252aE5ROCIsIm5iZiI6MTQ0ODE2NjcyMiwiYXVkIjoiYnRNSUFXbkpEQzdnUF85SzhfQzBxUjI2VWlNQmx5b2UifQ.CBm5HucgCKwOi-MkWdLKaJ9NMMnfCpHo1gjQ17VkYRFgMDanxPeQYjDZUhWUlKsb3wJmYdWs5CYeVbeFcKm_huIC5DmjNJ4QRODx3-XgetKdfHBwAwuWkUypTfMswOlWV1JONmZur9YBTHHqBIqLbcNKKj7HUtvMvQv0w-Dm29XuSoJtIMnQkDzBqyxUGQjkWcv1WjSqJAsFDUo_m8S6bSSWOR7xQFhQCm4gXH_wZ3JKaPv6bqOkfyUfwZ5YqFZzT5ybWhd6ZoLcUiFRsBZPfAqPA0WgHIdrdeS2DKGcrjboBjL4MOjS9yT5BWdT59RrBZQy19650Q1MqRxf1fSW_A")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		checkError(err)

		defer resp.Body.Close()
		contents, errmesg := ioutil.ReadAll(resp.Body)
		checkError(errmesg)

		json.Unmarshal([]byte(contents), &respUber1)

		jsonStr = []byte(`{"status": "accepted"}`)

		var buffer bytes.Buffer
		buffer.WriteString("https://sandbox-api.uber.com/v1/sandbox/requests?")
		buffer.WriteString(respUber1.RequestID)
		urlreq = buffer.String()
		req, err = http.NewRequest("POST", urlreq, bytes.NewBuffer(jsonStr))
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsImhpc3RvcnkiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiZjQwMDI4ZjEtNmVmMy00N2QxLWFiOTMtNzlkYWNhNWQ2ZmRhIiwiaXNzIjoidWJlci11czEiLCJqdGkiOiI2NWJmYmQ4NS0wNWZjLTQ2MzUtODRjYS00YzIyOGI1YjgyNTMiLCJleHAiOjE0NTA3NTg4MTMsImlhdCI6MTQ0ODE2NjgxMiwidWFjdCI6ImVFcW40WXRLMmc4NHZOcWZCVUNOdkVQc252aE5ROCIsIm5iZiI6MTQ0ODE2NjcyMiwiYXVkIjoiYnRNSUFXbkpEQzdnUF85SzhfQzBxUjI2VWlNQmx5b2UifQ.CBm5HucgCKwOi-MkWdLKaJ9NMMnfCpHo1gjQ17VkYRFgMDanxPeQYjDZUhWUlKsb3wJmYdWs5CYeVbeFcKm_huIC5DmjNJ4QRODx3-XgetKdfHBwAwuWkUypTfMswOlWV1JONmZur9YBTHHqBIqLbcNKKj7HUtvMvQv0w-Dm29XuSoJtIMnQkDzBqyxUGQjkWcv1WjSqJAsFDUo_m8S6bSSWOR7xQFhQCm4gXH_wZ3JKaPv6bqOkfyUfwZ5YqFZzT5ybWhd6ZoLcUiFRsBZPfAqPA0WgHIdrdeS2DKGcrjboBjL4MOjS9yT5BWdT59RrBZQy19650Q1MqRxf1fSW_A")
		req.Header.Set("Content-Type", "application/json")

		client = &http.Client{}
		resp, err = client.Do(req)
		checkError(err)

		resp2.ID = resultResponse.ID
		resp2.NextDestinationLocationID = resultResponse.StartingFromLocationID
		resp2.StartingFromLocationID = resultResponse.StartingFromLocationID
		resp2.TotalDistance = resultResponse.TotalDistance
		resp2.TotalUberCosts = resultResponse.TotalUberCosts
		resp2.TotalUberDuration = resultResponse.TotalUberDuration
		resp2.UberWaitTimeEta = respUber1.Eta

		for i := 0; i < len(resultResponse.BestRouteLocationIds); i++ {
			resp2.BestRouteLocationIds = append(resp2.BestRouteLocationIds, resultResponse.BestRouteLocationIds[i])
		}

		resp2.Status = "completed"
		// Marshal provided interface into JSON structure
		uj, _ := json.Marshal(resp2)

		// Write content-type, statuscode, payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, "%s", uj)

	} else if t == len(resultResponse.BestRouteLocationIds)-1 {
		err = c.FindId(resultResponse.BestRouteLocationIds[t-1]).One(&responseDB1)
		startLatitude := strconv.FormatFloat(responseDB1.Coordinate.Lat, 'g', -1, 64)
		startLongitude := strconv.FormatFloat(responseDB1.Coordinate.Lng, 'g', -1, 64)

		err = c.FindId(resultResponse.BestRouteLocationIds[t]).One(&responseDB1)
		endLatitude := strconv.FormatFloat(responseDB1.Coordinate.Lat, 'g', -1, 64)
		endLongitude := strconv.FormatFloat(responseDB1.Coordinate.Lng, 'g', -1, 64)
		var jsonStr = []byte(`{"start_latitude":"` + startLatitude + `","start_longitude":"` + startLongitude + `","end_latitude":"` + endLatitude + `","end_longitude":"` + endLongitude + `","product_id":"a1111c8c-c720-46c3-8534-2fcdd730040d"}`)

		urlreq := "https://sandbox-api.uber.com/v1/requests"
		req, err := http.NewRequest("POST", urlreq, bytes.NewBuffer(jsonStr))
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsImhpc3RvcnkiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiZjQwMDI4ZjEtNmVmMy00N2QxLWFiOTMtNzlkYWNhNWQ2ZmRhIiwiaXNzIjoidWJlci11czEiLCJqdGkiOiI2NWJmYmQ4NS0wNWZjLTQ2MzUtODRjYS00YzIyOGI1YjgyNTMiLCJleHAiOjE0NTA3NTg4MTMsImlhdCI6MTQ0ODE2NjgxMiwidWFjdCI6ImVFcW40WXRLMmc4NHZOcWZCVUNOdkVQc252aE5ROCIsIm5iZiI6MTQ0ODE2NjcyMiwiYXVkIjoiYnRNSUFXbkpEQzdnUF85SzhfQzBxUjI2VWlNQmx5b2UifQ.CBm5HucgCKwOi-MkWdLKaJ9NMMnfCpHo1gjQ17VkYRFgMDanxPeQYjDZUhWUlKsb3wJmYdWs5CYeVbeFcKm_huIC5DmjNJ4QRODx3-XgetKdfHBwAwuWkUypTfMswOlWV1JONmZur9YBTHHqBIqLbcNKKj7HUtvMvQv0w-Dm29XuSoJtIMnQkDzBqyxUGQjkWcv1WjSqJAsFDUo_m8S6bSSWOR7xQFhQCm4gXH_wZ3JKaPv6bqOkfyUfwZ5YqFZzT5ybWhd6ZoLcUiFRsBZPfAqPA0WgHIdrdeS2DKGcrjboBjL4MOjS9yT5BWdT59RrBZQy19650Q1MqRxf1fSW_A")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		checkError(err)

		defer resp.Body.Close()
		contents, errmesg := ioutil.ReadAll(resp.Body)
		checkError(errmesg)

		json.Unmarshal([]byte(contents), &respUber1)

		jsonStr = []byte(`{"status": "accepted"}`)

		var buffer bytes.Buffer
		buffer.WriteString("https://sandbox-api.uber.com/v1/sandbox/requests?")
		buffer.WriteString(respUber1.RequestID)
		urlreq = buffer.String()
		req, err = http.NewRequest("POST", urlreq, bytes.NewBuffer(jsonStr))
		req.Header.Set("Authorization", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicHJvZmlsZSIsImhpc3RvcnkiLCJoaXN0b3J5X2xpdGUiXSwic3ViIjoiZjQwMDI4ZjEtNmVmMy00N2QxLWFiOTMtNzlkYWNhNWQ2ZmRhIiwiaXNzIjoidWJlci11czEiLCJqdGkiOiI2NWJmYmQ4NS0wNWZjLTQ2MzUtODRjYS00YzIyOGI1YjgyNTMiLCJleHAiOjE0NTA3NTg4MTMsImlhdCI6MTQ0ODE2NjgxMiwidWFjdCI6ImVFcW40WXRLMmc4NHZOcWZCVUNOdkVQc252aE5ROCIsIm5iZiI6MTQ0ODE2NjcyMiwiYXVkIjoiYnRNSUFXbkpEQzdnUF85SzhfQzBxUjI2VWlNQmx5b2UifQ.CBm5HucgCKwOi-MkWdLKaJ9NMMnfCpHo1gjQ17VkYRFgMDanxPeQYjDZUhWUlKsb3wJmYdWs5CYeVbeFcKm_huIC5DmjNJ4QRODx3-XgetKdfHBwAwuWkUypTfMswOlWV1JONmZur9YBTHHqBIqLbcNKKj7HUtvMvQv0w-Dm29XuSoJtIMnQkDzBqyxUGQjkWcv1WjSqJAsFDUo_m8S6bSSWOR7xQFhQCm4gXH_wZ3JKaPv6bqOkfyUfwZ5YqFZzT5ybWhd6ZoLcUiFRsBZPfAqPA0WgHIdrdeS2DKGcrjboBjL4MOjS9yT5BWdT59RrBZQy19650Q1MqRxf1fSW_A")
		req.Header.Set("Content-Type", "application/json")

		client = &http.Client{}
		resp, err = client.Do(req)
		checkError(err)

		resp2.ID = resultResponse.ID
		resp2.NextDestinationLocationID = resultResponse.BestRouteLocationIds[t]
		resp2.StartingFromLocationID = resultResponse.StartingFromLocationID
		resp2.TotalDistance = resultResponse.TotalDistance
		resp2.TotalUberCosts = resultResponse.TotalUberCosts
		resp2.TotalUberDuration = resultResponse.TotalUberDuration
		resp2.UberWaitTimeEta = respUber1.Eta

		for i := 0; i < len(resultResponse.BestRouteLocationIds); i++ {
			resp2.BestRouteLocationIds = append(resp2.BestRouteLocationIds, resultResponse.BestRouteLocationIds[i])
		}
		t = t + 1
		resp2.Status = "requesting"
		// Marshal provided interface into JSON structure
		uj, _ := json.Marshal(resp2)

		// Write content-type, statuscode, payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, "%s", uj)
	}
	fmt.Println(t)

}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	r := httprouter.New()

	r.POST("/trips", PlanTrip)
	r.GET("/trips/:tripid", GetTripDetails)
	r.PUT("/trips/:tripid/request", RequestACar)

	http.ListenAndServe("localhost:6000", r)
}
