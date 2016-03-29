package main
import (
    "fmt"
    "github.com/julienschmidt/httprouter"
    "net/http"
    "encoding/json"
    "gopkg.in/mgo.v2/bson"
    "gopkg.in/mgo.v2"
    "strconv"
    "net/url"
    "math/rand"
    "bytes"
    "time"
)

//Structure for request json
type Request struct {
    Starting_from_location_id string `json:"id" bson:"id"`
    Location_ids []string `json:Location_ids`
}

//Structure for response json
type Response struct {
    Id string `json:"id" "bson":"id"`
    Name string `json:"name" bson:"name"`
    Address string `json:"address" bson:"address"`
    City string `json:"city" bson:"city"`
    State string `json:"state" bson:"state"`
    Zip string `json:"zip" bson:"zip"`
    Coordinates  interface{} `json:"coordinate" bson:"coordinate"`
    Location_ids []string `json:Location_ids`
}

//Structure to point the current location
type CurrentLocationTracker struct {
    Tracker int `json:"tracker" "bson":"tracker"`
}

//Temporary structure to store Uber Details Data
type Uberdata struct {
    End_id  string
    Duration  float64
    Distance  float64
    High_Estimate float64
}

//Temporary structure
type Message struct {
    Start_latitude string `json:"start_latitude"`
    Start_longitude string `json:"start_longitude"`
    End_latitude string `json:"end_latitude"`
    End_longitude string `json:"end_longitude"`
    Product_id string `json:"product_id"`
}

//Structure for TripPlanner json
type TripPlanner struct {
    Id string `json:"id" "bson":"id"`
    Status string `json:"status" "bson":"status"`
    Starting_from_location_id string `json:"starting_from_location_id" "bson":"starting_from_location_id"`
    Best_route_location_ids []string `json:"best_route_location_ids" "bson":"best_route_location_ids"`
    Total_uber_costs float64 `json:"total_uber_costs" "bson":"total_uber_costs"`
    Total_uber_duration float64 `json:"total_uber_duration" "bson":"total_uber_duration"`
    Total_distance float64 `json:"total_distance" "bson":"total_distance"`
}

//Structure for TripPlanner Request json
type PutTripPlanner struct {
    Id string `json:"id" "bson":"id"`
    Status string `json:"status" "bson":"status"`
    Starting_from_location_id string `json:"starting_from_location_id" "bson":"starting_from_location_id"`
    Next_destination_location_id string `json:"next_destination_location_id" "bson":"next_destination_location_id"`
    Best_route_location_ids []string `json:"best_route_location_ids" "bson":"best_route_location_ids"`
    Total_uber_costs float64 `json:"total_uber_costs" "bson":"total_uber_costs"`
    Total_uber_duration float64 `json:"total_uber_duration" "bson":"total_uber_duration"`
    Total_distance float64 `json:"total_distance" "bson":"total_distance"`
    Uber_wait_time_eta int `json:"uber_wait_time_eta" "bson":"uber_wait_time_eta"`
}

var mgoSession *mgo.Session
//Global constant to point current location
var MY_CONSTANT int

//function for POST request
func myPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

    var distance, duration, high_estimate, total_uber_duration, total_distance, total_uber_costs float64
    
    req := Request{}
    var res []Response
    res = append(res, Response{})

    json.NewDecoder(r.Body).Decode(&req)

    res[0].Id = req.Starting_from_location_id
    res[0].Location_ids = req.Location_ids

    res = GetLocation(res[0].Location_ids, res[0].Id)
    
    var myBestRoute []Response
    myBestRoute = append(myBestRoute, Response{})
    myBestRoute = GetBestRoute(res)
    var bestRouteLocation [] string

    for index, _ := range myBestRoute {
        bestRouteLocation = append(bestRouteLocation, myBestRoute[index].Id)
        startLocLat := myBestRoute[index].Coordinates.(bson.M)["lat"].(float64)
        startLocLong := myBestRoute[index].Coordinates.(bson.M)["lng"].(float64)

        //check whether the trip is over? If yes, return to starting location
        if index != len(res)-1 {
            endLocLat := myBestRoute[index+1].Coordinates.(bson.M)["lat"].(float64)
            endLocLong := myBestRoute[index+1].Coordinates.(bson.M)["lng"].(float64)
            //call uber api to get price estimates
            duration, distance, high_estimate, _ = GetPriceEstimates(startLocLat, startLocLong, endLocLat, endLocLong)
        }

        if index == len(res)-1 {
            //call uber api to get price estimates
            duration, distance, high_estimate, _ = GetPriceEstimates(startLocLat, startLocLong, myBestRoute[0].Coordinates.(bson.M)["lat"].(float64), myBestRoute[0].Coordinates.(bson.M)["lng"].(float64))
        }
        total_uber_costs = total_uber_costs + high_estimate
        total_uber_duration = total_uber_duration + duration
        total_distance = total_distance + distance
    }

    res[0].Location_ids = req.Location_ids
    tripPlannerResponse := TripPlanner{}
    
    //generating random id by using system time
    rand.Seed(time.Now().UTC().UnixNano())
    MY_CONSTANT = rand.Intn(5000)
    tripPlannerResponse.Id = strconv.Itoa(MY_CONSTANT)

    //setting status
    tripPlannerResponse.Status = "planning"

    bestRouteLocation = append(bestRouteLocation[:0], bestRouteLocation[1:]...)
    tripPlannerResponse.Best_route_location_ids = bestRouteLocation
    tripPlannerResponse.Total_distance = total_distance
    tripPlannerResponse.Total_uber_costs = total_uber_costs
    tripPlannerResponse.Total_uber_duration = total_uber_duration
    tripPlannerResponse.Starting_from_location_id = req.Starting_from_location_id
    
    //insert in mongo
    mgoSession, err := mgo.Dial("mongodb://user:user@ds043962.mongolab.com:43962/gorest")
    // Check if connection error, is mongo running?
    if err != nil {
        panic(err)
    }
    mgoSession.DB("gorest").C("TripPlanner").Insert(tripPlannerResponse)

    data, _ := json.MarshalIndent(tripPlannerResponse, "", "\t")

    if err := mgoSession.DB("gorest").C("TripPlanner").Update(bson.M{"id": strconv.Itoa(MY_CONSTANT)}, bson.M{"$set": bson.M{"tracker": 0}}); err != nil {
        panic(err)
    }

    // Write content-type, statuscode, payload
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    fmt.Fprintf(w, "%s", data)
}

func GetBestRoute(routes []Response) []Response { 

    var bestroute []Response
    var uberdata []Uberdata
    bestroute = append(bestroute, Response{})
    
    //assign starting location as first location
    bestroute[0] = routes[0]

    //delete first element from routes
    routes = RemoveThisId(routes, routes[0].Id)

    NearestlocationId := ""

    lenroutes := len(routes)
    for i := 0; i < lenroutes; i++ {
        //fmt.Println(i)
        
        startLocLat := bestroute[i].Coordinates.(bson.M)["lat"].(float64)
        startLocLong := bestroute[i].Coordinates.(bson.M)["lng"].(float64)
        
        uberdata = uberdata[:0]
        index_uber := 0
        for index, _ := range routes {

            endLocLat := routes[index].Coordinates.(bson.M)["lat"].(float64)
            endLocLong := routes[index].Coordinates.(bson.M)["lng"].(float64)
            uberdata = append(uberdata, Uberdata{})

            uberdata[index_uber].Duration, uberdata[index_uber].Distance, uberdata[index_uber].High_Estimate, _ = GetPriceEstimates(startLocLat, startLocLong, endLocLat, endLocLong)
            uberdata[index_uber].End_id = routes[index].Id
            index_uber++
        }
        NearestlocationId = GetLowestId(uberdata)
        bestroute = CreateBestRoute(routes, NearestlocationId, bestroute)
        routes = RemoveThisId(routes, NearestlocationId)
    }
    
    //return routes add later
    return bestroute
}

func CreateBestRoute(routes []Response, NearestlocationId string, bestroute []Response) []Response {
    index1 := len(bestroute)
    bestroute = append(bestroute, Response{})
    for index, _ := range routes {
        if NearestlocationId == routes[index].Id {
            bestroute[index1] = routes[index]
        }
    }
    return bestroute
}

func RemoveThisId(routes []Response, NearestlocationId string) []Response {

    for index, _ := range routes {
        //overwrite if found
        if NearestlocationId == routes[index].Id {
            routes = append(routes[:index], routes[index+1:]...)
            break
        }
    }
    return routes
}

func GetLowestId(uberdata []Uberdata) string {
    min := 9999.00
    tempId := ""
    for index, _ := range uberdata {
        if uberdata[index].High_Estimate < min {
            min = uberdata[index].High_Estimate
            tempId = uberdata[index].End_id
        }
    }
    return tempId
}

func GetLocation(location_ids []string, starting_from_location_id string) []Response {
    var number []int
    var startLoc int

    //convert to integer
    startLoc, _ = strconv.Atoi(starting_from_location_id)
    number = append(number, startLoc)
    var res []Response
    
    for _, element := range location_ids {
        temp, _ := strconv.Atoi(element)
        number = append(number, temp)
    }
    
    for index, _ := range number {
        res = append(res, FetchFromMongo(number[index]))
        temp_location := strconv.Itoa(number[index])
        res[index].Id = temp_location
    }
    return res
}

func FetchFromMongo(location int) Response {
    res := Response{}
    mgoSession, err := mgo.Dial("mongodb://user:user@ds043962.mongolab.com:43962/gorest")
    if err != nil {
        panic(err)
    }
    
    // Fetch data corresponding to id
    if err := mgoSession.DB("gorest").C("user").Find(bson.M{"id":location}).One(&res); err!=nil{
        panic(err)
    }
    return res
}

func GetPriceEstimates(start_latitude float64, start_longitude float64, end_latitude float64, end_longitude float64) (float64, float64, float64, string) {
    var Url *url.URL
    Url, err := url.Parse("https://sandbox-api.uber.com")
    if err != nil {
        panic("Error Panic")
    }
    Url.Path += "/v1/estimates/price"
    
    //fetching values
    parameters := url.Values{}
    start_lat := strconv.FormatFloat(start_latitude, 'f', 6, 64)
    start_long := strconv.FormatFloat(start_longitude, 'f', 6, 64)
    end_lat := strconv.FormatFloat(end_latitude, 'f', 6, 64)
    end_long := strconv.FormatFloat(end_longitude, 'f', 6, 64)
    
    //creating object
    parameters.Add("server_token", "8_RktM1qhU7Vg1MKODdMQmYaoWdPALoKLNMFAvku")
    parameters.Add("start_latitude", start_lat)
    parameters.Add("start_longitude", start_long)
    parameters.Add("end_latitude", end_lat)
    parameters.Add("end_longitude", end_long)
    Url.RawQuery = parameters.Encode()

    //fetch uber api
    res, err := http.Get(Url.String())
    if err != nil {
        panic("Error Panic")
    }
    defer res.Body.Close()

    var v map[string]interface{}

    dec := json.NewDecoder(res.Body)
    if err := dec.Decode(&v); err != nil {
        fmt.Println("ERROR: " + err.Error())
    }

    //fetch data from json
    duration := v["prices"].([]interface{})[0].(map[string]interface{})["duration"].(float64)
    distance := v["prices"].([]interface{})[0].(map[string]interface{})["distance"].(float64)
    product_id := v["prices"].([]interface{})[0].(map[string]interface{})["product_id"].(string)
    high_estimate := v["prices"].([]interface{})[0].(map[string]interface{})["high_estimate"].(float64)

    return duration, distance, high_estimate, product_id
}


func myGet(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
    
    //fetching id from parameters
    id := p.ByName("id")

    //connecting to Mongo
    mgoSession, err := mgo.Dial("mongodb://user:user@ds043962.mongolab.com:43962/gorest");
    if err!=nil{
        panic(err)
    }

    //creating a TripPlanner object
    res := TripPlanner{}

    // Fetch record corresponding to id
    if err := mgoSession.DB("gorest").C("TripPlanner").Find(bson.M{"id":id}).One(&res); err!=nil{
        w.WriteHeader(404)
        return
    }

    // Marshal provided interface into JSON structure
    data, _ := json.MarshalIndent(res, "", "\t")

    // Write content-type, statuscode, payload
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    fmt.Fprintf(w, "%s", data)
}

func FetchETA(nextLocation string, currentLocation string) float64 {
    
    var responseArray []Response
    responseArray = append(responseArray, Response{})
    
    responseArray = GetLocation([]string{nextLocation}, currentLocation)

    //converting received data to float
    startLocLat := responseArray[0].Coordinates.(bson.M)["lat"].(float64)
    startLocLong := responseArray[0].Coordinates.(bson.M)["lng"].(float64)
    endLocLat := responseArray[1].Coordinates.(bson.M)["lat"].(float64)
    endLocLong := responseArray[1].Coordinates.(bson.M)["lng"].(float64)
    
    //call uber api
    _, _, _, product_id := GetPriceEstimates(startLocLat, startLocLong, endLocLat, endLocLong)

    v1 := Message{
        Start_latitude:  strconv.FormatFloat(startLocLat, 'f', 6, 64),
        Start_longitude: strconv.FormatFloat(startLocLong, 'f', 6, 64),
        End_latitude:    strconv.FormatFloat(endLocLat, 'f', 6, 64),
        End_longitude:   strconv.FormatFloat(endLocLong, 'f', 6, 64),
        Product_id:      product_id,
    }

    jsonStr, _ := json.Marshal(v1)
    
    client := &http.Client{}
    r, err := http.NewRequest("POST", "https://sandbox-api.uber.com/v1/requests", bytes.NewBuffer(jsonStr)) 
    
    if err != nil {
        panic(err)
    }

    r.Header.Set("Content-Type", "application/json")
    r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
    r.Header.Add("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicmVxdWVzdCJdLCJzdWIiOiIwM2VjN2FkOC0wNmJlLTRmMzctYTg5My1jZTJmMDJiNTNmZWIiLCJpc3MiOiJ1YmVyLXVzMSIsImp0aSI6IjJiYWEyOWRlLTc1YzctNDc5Yy1iMmFiLTliOTMzMTM5MDBkNCIsImV4cCI6MTQ1MDYyODIyNywiaWF0IjoxNDQ4MDM2MjI2LCJ1YWN0IjoiVEtkcU1xOEIxZ29sMGRSQ0ZoaThUOTJyV3pFYWc4IiwibmJmIjoxNDQ4MDM2MTM2LCJhdWQiOiJmeVVNVV9XR2t1dGtmUUQyVXpmdld5UFlNTGRHa3hLMCJ9.fDenlm7rBiTaZJsWtmdf6B-tClnXzckh4DaQRtFU1W_60nuYb2tym093VNen8HWuuUnGd-Lcb_Gpiv-hfOe66ZrCL-Ij1YwdmXqfEcc-UMvVFIRrNe6aIshjNHv7L72buJzfQeoSE66aKkSASJUkoBTGWFoiOBgnZSUBW_Il7JRAM_gWR2juvNyJyQO1lN-97cFsSEwwMmsPkPBKAcmtn5veaWLU57McOVjFxImwd2dGczgAljzkuQBasD9pWnTsKG7vBl7R7HYEogGT6sy5Qyhm3VgOLt4HKWl0gRxANR6qUlx--Y-hXfz_Z71KBjdR_qJEH7VFIAAgykj84S8LPg")

    resp, _ := client.Do(r)
    defer resp.Body.Close()
    
    var v map[string]interface{}
    dec := json.NewDecoder(resp.Body)
    if err := dec.Decode(&v); err != nil {
        fmt.Println("ERROR: " + err.Error())
    }

    return v["eta"].(float64)
}

func myPut(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

    var currentLoc, nextLoc string

    //fetching id from parameters
    id := p.ByName("id")

    //mongo connect
    mgoSession, err := mgo.Dial("mongodb://user:user@ds043962.mongolab.com:43962/gorest")
    if err != nil {
        panic(err)
    }

    //Retreiving the tracker to check current location
    currLocTracker := CurrentLocationTracker{}

    //search for id in database
    if err := mgoSession.DB("gorest").C("TripPlanner").Find(bson.M{"id": id}).One(&currLocTracker); err != nil {
        w.WriteHeader(404)
        return
    }

    putTripPlanner := PutTripPlanner{}
    if err := mgoSession.DB("gorest").C("TripPlanner").Find(bson.M{"id": id}).One(&putTripPlanner); err != nil {
        w.WriteHeader(404)
        return
    }

    //if current location == starting location
    if currLocTracker.Tracker == 0 {
        currentLoc = putTripPlanner.Starting_from_location_id
        nextLoc = putTripPlanner.Best_route_location_ids[0]
        if err := mgoSession.DB("gorest").C("TripPlanner").Update(bson.M{"id": id}, bson.M{"$set": bson.M{"current_location": currentLoc, "next_destination_location_id": nextLoc}}); err != nil {
            w.WriteHeader(404)
        }
        //incrementing currLocTracker
        if err := mgoSession.DB("gorest").C("TripPlanner").Update(bson.M{"id": id}, bson.M{"$set": bson.M{"status": "requesting"}}); err != nil {
            w.WriteHeader(404)
            return
             }
        currLocTracker.Tracker += 1

    } else {

        //if last location visited, next location = first location
        if currLocTracker.Tracker == len(putTripPlanner.Best_route_location_ids) {
            currentLoc = putTripPlanner.Next_destination_location_id
            nextLoc = putTripPlanner.Starting_from_location_id
        } else if currLocTracker.Tracker > len(putTripPlanner.Best_route_location_ids) {
            //if trip completed, print status = finished
            if err := mgoSession.DB("gorest").C("TripPlanner").Update(bson.M{"id": id}, bson.M{"$set": bson.M{"status": "completed"}}); err != nil {
                w.WriteHeader(404)
                return
            }            
        } else {
            currentLoc = putTripPlanner.Next_destination_location_id
            nextLoc = putTripPlanner.Best_route_location_ids[currLocTracker.Tracker]
        }
    
        //update the record in database
        if err := mgoSession.DB("gorest").C("TripPlanner").Update(bson.M{"id": id}, bson.M{"$set": bson.M{"current_location": currentLoc, "next_destination_location_id": nextLoc}}); err != nil {
            w.WriteHeader(404)
            return
        }
        //incrementing currLocTracker
        currLocTracker.Tracker += 1
    }

    //updating the currLocTracker
    if err := mgoSession.DB("gorest").C("TripPlanner").Update(bson.M{"id": id}, bson.M{"$set": bson.M{"tracker": currLocTracker.Tracker}}); err != nil {
        fmt.Println("Insertion Failed")
        panic(err)
    }

    var eta float64
    //fetch ETA
    if (currLocTracker.Tracker-1) < (len(putTripPlanner.Best_route_location_ids)+1){
        eta = FetchETA(nextLoc, currentLoc)
    }else {
        eta = 0
    }

    if err := mgoSession.DB("gorest").C("TripPlanner").Update(bson.M{"id": id}, bson.M{"$set": bson.M{"uber_wait_time_eta": eta}}); err != nil {
        panic(err)
    }

    //Preparing the PUT RESPONSE
    if err := mgoSession.DB("gorest").C("TripPlanner").Find(bson.M{"id": id}).One(&putTripPlanner); err != nil {
        w.WriteHeader(404)
        return
    }

    data, _ := json.MarshalIndent(putTripPlanner, "", "\t")
    
    // Write content-type, statuscode, payload
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    fmt.Fprintf(w, "%s", data)
}

func main() {
    
    //global constants
    MY_CONSTANT = 1000
    
    //creating a new httprouter instance
    mux := httprouter.New()

    //invoking functions for specific paths
    mux.GET("/trips/:id", myGet)
    mux.POST("/trips", myPost)
    mux.PUT("/trips/:id/request", myPut)
    
    //activating server at port 8080
    server := http.Server{
        Addr:    "0.0.0.0:8080",
        Handler: mux,
    }    
    //listening
    server.ListenAndServe()
}