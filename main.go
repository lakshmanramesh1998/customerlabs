package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.ForwardedByClientIP = true
	r.SetTrustedProxies([]string{"127.0.0.1"})
	r.POST("/", ProcessReq())
	r.Run(":8080")
}

type Values struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type Attribute map[string]SubAttributes

type SubAttributes map[string]Values

type Response struct {
	Event       string        `json:"event"`
	EventType   string        `json:"event_type"`
	AppID       string        `json:"app_id"`
	UserID      string        `json:"user_id"`
	MessageID   string        `json:"message_id"`
	PageTitle   string        `json:"page_title"`
	PageUrl     string        `json:"page_url"`
	BrowserLang string        `json:"browser_language"`
	ScreenSize  string        `json:"screen_size"`
	Attributes  SubAttributes `json:"attributes"`
	Traits      SubAttributes `json:"traits"`
}

func extractAttributes(prefix string, req map[string]string, limit int) SubAttributes {
	subAttr := SubAttributes{}
	for i := 1; i <= limit; i++ {
		key := req[prefix+"k"+strconv.Itoa(i)]
		value := Values{
			Value: req[prefix+"v"+strconv.Itoa(i)],
			Type:  req[prefix+"t"+strconv.Itoa(i)],
		}
		subAttr[key] = value
	}
	return subAttr
}

func worker(reqChan chan map[string]string, response chan<- Response, maxAtrk, maxUatrk int) {
	for req := range reqChan {
		res := Response{
			Event:       req["ev"],
			EventType:   req["et"],
			AppID:       req["id"],
			UserID:      req["uid"],
			MessageID:   req["mid"],
			PageTitle:   req["t"],
			PageUrl:     req["p"],
			BrowserLang: req["l"],
			ScreenSize:  req["sc"],
		}

		res.Attributes = extractAttributes("atr", req, maxAtrk)
		res.Traits = extractAttributes("uatr", req, maxUatrk)

		response <- res
	}
}

func ProcessReq() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req map[string]string
		reqChan := make(chan map[string]string)
		response := make(chan Response)
		err := c.BindJSON(&req)
		if err != nil {
			return
		}

		// Variables to store the highest numbers for atrk and uatrk
		maxAtrk := 0
		maxUatrk := 0

		re := regexp.MustCompile(`\d+`)

		// Iterate over the keys and find the strings with numbers
		for key, _ := range req {

			// Check if the key contains atrk or uatrk and update the max value accordingly
			if matched, _ := regexp.MatchString(`^(atrk|uatrk)\d+$`, key); matched {
				numStr := re.FindString(key)
				if num, err := strconv.Atoi(numStr); err == nil {
					if key[:4] == "atrk" && num > maxAtrk {
						maxAtrk = num
					} else if key[:5] == "uatrk" && num > maxUatrk {
						maxUatrk = num
					}
				}
			}
		}

		go worker(reqChan, response, maxAtrk, maxUatrk)
		reqChan <- req
		go func() {
			for result := range response {
				// send the result to the url
				uri := "https://webhook.site/df802475-c06f-410a-8ae3-aefcf2e36dd4"
				data, err := json.Marshal(result)
				if err != nil {
					fmt.Println("Error marshalling JSON:", err)
					return
				}

				// Create a new HTTP request
				request, err := http.NewRequest("POST", uri, bytes.NewBuffer(data))
				if err != nil {
					fmt.Println("Error creating request:", err)
					return
				}

				// Set request headers if needed
				request.Header.Set("Content-Type", "application/json")

				// Create an HTTP client
				client := &http.Client{}

				// Send the request
				resp, err := client.Do(request)
				if err != nil {
					fmt.Println("Error sending request:", err)
					c.JSON(http.StatusBadRequest, err.Error())
					return
				}
				defer resp.Body.Close()

				c.JSON(http.StatusOK, result)
			}
		}()
	}
}
