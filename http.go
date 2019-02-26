package main

import (
	"encoding/json"
	"github.com/labstack/gommon/log"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func (s *Shard) GatewayBot() (st *GatewayBotResponse, err error) {

	client := &http.Client{Timeout: time.Second * 10}

	req, err := http.NewRequest("GET", APIBase+"/gateway/bot", nil)

	req.Header.Set("Authorization", "Bot "+s.Token)

	response, err := client.Do(req)
	if err != nil {
		log.Fatal("Error getting Gateway data: ", err)
		return
	}

	defer func() {
		err := response.Body.Close()
		if err != nil {
			return
		}
	}()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal("error reading gateway response body ", err)
	}

	err = json.Unmarshal(body, &st)
	if err != nil {
		return
	}

	// Ensure the gateway always has a trailing slash.
	// MacOS will fail to connect if we add query params without a trailing slash on the base domain.
	if !strings.HasSuffix(st.URL, "/") {
		st.URL += "/"
	}

	return
}
