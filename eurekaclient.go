/**
The MIT License (MIT)

Copyright (c) 2016 ErikL

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
package eeureka

import (
	"encoding/json"
	"fmt"
	"github.com/twinj/uuid"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var instanceId string
var discoveryServerUrl = "http://192.168.1.25:9000"
var ports  = ""
var securePorts = ""

var regTpl = `{
  "instance": {
    "hostName":"${ipAddress}",
    "app":"${appName}",
    "ipAddr":"${ipAddress}",
    "vipAddress":"${appName}",
    "status":"UP",
    "port": {
      "$":${port},
      "@enabled": true
    },
    "securePort": {
      "$":${securePort},
      "@enabled": false
    },
    "homePageUrl" : "http://${ipAddress}:${port}/",
    "statusPageUrl": "http://${ipAddress}:${port}/info",
    "healthCheckUrl": "http://${ipAddress}:${port}/health",
    "dataCenterInfo" : {
      "@class":"com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
      "name": "MyOwn"
    },
    "metadata": {
      "instanceId" : "${appName}:${instanceId}"
    }
  }
}`

/**
 * Registers this application at the Eureka server at @eurekaUrl as @appName running on port(s) @port and/or @securePort.
 */
func RegisterAt(eurekaUrl string, appName string, port string, securePort string) {
	discoveryServerUrl = eurekaUrl
	ports = port
	securePorts = securePort
	Register(appName, port, securePort, true)
}

/**
  Register the application at the default eurekaUrl.
*/
func Register(appName string, port string, securePort string, init bool) {
	//判断是否是重连请求 true为初始化，false为重新链接
	if init {
		instanceId = getUUID()
	}

	tpl := string(regTpl)
	tpl = strings.Replace(tpl, "${ipAddress}", getLocalIP(), -1)
	tpl = strings.Replace(tpl, "${port}", port, -1)
	tpl = strings.Replace(tpl, "${securePort}", securePort, -1)
	tpl = strings.Replace(tpl, "${instanceId}", instanceId, -1)
	tpl = strings.Replace(tpl, "${appName}", appName, -1)
	//fmt.Println(tpl)
	// Register.
	registerAction := HttpAction{
		Url:         discoveryServerUrl + "/eureka/apps/" + appName,
		Method:      "POST",
		ContentType: "application/json;charset=UTF-8",
		Body:        tpl,
	}
	var result bool
	for {
		result = doHttpRequest(registerAction)
		if result {
			fmt.Println("Registration OK")
			handleSigterm(appName)
			if init {
				go StartHeartbeat(appName)
			}
			break
		} else {
			fmt.Println("Registration attempt of " + appName + " failed...")
			time.Sleep(time.Second * 5)
		}
	}

}

/**
 * Given the supplied appName, this func queries the Eureka API for instances of the appName and returns
 * them as a EurekaApplication struct.
 */
func GetServiceInstances(appName string) ([]EurekaInstance, error) {
	var m EurekaServiceResponse
	fmt.Println("Querying eureka for instances of " + appName + " at: " + discoveryServerUrl + "/eureka/apps/" + appName)
	queryAction := HttpAction{
		Url:         discoveryServerUrl + "/eureka/apps/" + appName,
		Method:      "GET",
		Accept:      "application/json;charset=UTF-8",
		ContentType: "application/json;charset=UTF-8",
	}
	log.Println("Doing queryAction using URL: " + queryAction.Url)
	bytes, err := executeQuery(queryAction)
	if err != nil {
		return nil, err
	} else {
		fmt.Println("Got instances response from Eureka:\n" + string(bytes))
		err := json.Unmarshal(bytes, &m)
		if err != nil {
			fmt.Println("Problem parsing JSON response from Eureka: " + err.Error())
			return nil, err
		}
		return m.Application.Instance, nil
	}
}

// Experimental, untested.
func GetServices() ([]EurekaApplication, error) {
	var m EurekaApplicationsRootResponse
	fmt.Println("Querying eureka for services at: " + discoveryServerUrl + "/eureka/apps")
	queryAction := HttpAction{
		Url:         discoveryServerUrl + "/eureka/apps",
		Method:      "GET",
		Accept:      "application/json;charset=UTF-8",
		ContentType: "application/json;charset=UTF-8",
	}
	log.Println("Doing queryAction using URL: " + queryAction.Url)
	bytes, err := executeQuery(queryAction)
	if err != nil {
		return nil, err
	} else {
		fmt.Println("Got services response from Eureka:\n" + string(bytes))
		err := json.Unmarshal(bytes, &m)
		if err != nil {
			fmt.Println("Problem parsing JSON response from Eureka: " + err.Error())
			return nil, err
		}
		return m.Resp.Applications, nil
	}
}

// Start as goroutine, will loop indefinitely until application exits.
func StartHeartbeat(appName string) {
	for {
		time.Sleep(time.Second * 30)
		fmt.Println("我在运行")
		heartbeat(appName)
	}
}

func heartbeat(appName string) {
	heartbeatAction := HttpAction{
		Url:         discoveryServerUrl + "/eureka/apps/" + appName + "/" + getLocalIP() + ":" + appName + ":" + instanceId,
		Method:      "PUT",
		ContentType: "application/json;charset=UTF-8",
	}
	fmt.Println("Issuing heartbeat to " + heartbeatAction.Url)
	bool := doHttpRequest(heartbeatAction)
	if !bool {
		Register(appName, ports, securePorts, false)
	}
}

func deregister(appName string) {
	fmt.Println("Trying to deregister application " + appName + "...")
	// Deregister
	deregisterAction := HttpAction{
		Url:         discoveryServerUrl + "/eureka/apps/" + appName + "/" + getLocalIP(),
		ContentType: "application/json;charset=UTF-8",
		Method:      "DELETE",
	}
	doHttpRequest(deregisterAction)
	fmt.Println("Deregistered application " + appName + ", exiting. Check Eureka...")
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.To4().String()
			}
		}
	}
	panic("Unable to determine local IP address (non loopback). Exiting.")
	//return "192.168.1.25"
}

func getUUID() string {
	return uuid.NewV4().String()
}

func handleSigterm(appName string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		deregister(appName)
		os.Exit(1)
	}()
}
