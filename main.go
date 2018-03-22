package main

import (
	"github.com/alouca/gosnmp"
	"log"
	"fmt"
	"strings"
	_ "github.com/ziutek/mymysql/native"
	"github.com/tealeg/xlsx"
	"github.com/ziutek/mymysql/mysql"
	"errors"
	"io/ioutil"
	"encoding/json"
)

type Switch struct {
	ProductNum string
	SerialNum  string
	Hostname   string
}

var dataAsses struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func loadConfig() error {
	jsonData, err := ioutil.ReadFile("config.json")
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, &dataAsses)
}

var streamOfSwChanel chan Switch
var update chan int

func ipDecode(ip uint) string {
	return fmt.Sprintf("%d.%d.%d.%d", (ip>>24)%0xFF, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
}

func wrRow(data Switch, sheet *xlsx.Sheet) {
	row := sheet.AddRow()
	cell1 := row.AddCell()
	cell1.Value = data.ProductNum
	cell2 := row.AddCell()
	cell2.Value = data.SerialNum
	cell3 := row.AddCell()
	cell3.Value = ""
	cell4 := row.AddCell()
	cell4.Value = data.Hostname
}

func getdata(ip string) error {
	var sw Switch
	s, err := gosnmp.NewGoSNMP(ip, "public", gosnmp.Version2c, 1)
	if err != nil {
		return err
	}
	response, err := s.Walk(".1.3.6.1.2.1.47.1.1.1.1.2")
	if err != nil {
		return err
	}
	if len(response) == 0 {
		return errors.New("dlink")
	}
	sw.ProductNum = strings.Split(fmt.Sprintf("%v", response[0].Value), " ")[1]

	response, err = s.Walk("1.3.6.1.4.1.11.2.36.1.1.2.9")
	if err != nil {
		return err
	}
	if len(response) == 0 {
		return errors.New("dlink")
	}
	sw.SerialNum = fmt.Sprintf("%v", response[0].Value)

	response, err = s.Walk("1.3.6.1.2.1.1.5")
	if err != nil {
		return err
	}
	if len(response) == 0 {
		return errors.New("dlink")
	}
	sw.Hostname = fmt.Sprintf("%v", response[0].Value)
	streamOfSwChanel <- sw
	return nil
}

func waiter(num int) {
	var sw Switch
	counter := 0
	iteration := 1

	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Sheet1")
	if err != nil {
		fmt.Printf(err.Error())
	}

	for {
		select {
		case sw = <-streamOfSwChanel:
			wrRow(sw, sheet)
			counter++
			if counter%50 == 0 {
				err = file.Save(fmt.Sprintf("switches%d.xlsx", iteration))
				if err != nil {
					log.Println(err.Error())
				}
				iteration++
				file = xlsx.NewFile()
				sheet, err = file.AddSheet("Sheet1")
				if err != nil {
					fmt.Printf(err.Error())
				}
			}
			if counter >= num {
				err = file.Save(fmt.Sprintf("switches%d.xlsx", iteration))
				if err != nil {
					log.Println(err.Error())
				}
				return
			}
		case num = <-update:
			if counter >= num {
				err = file.Save(fmt.Sprintf("switches%d.xlsx", iteration))
				if err != nil {
					log.Println(err.Error())
				}
				return
			}
		}
	}
}

func main() {
	streamOfSwChanel = make(chan Switch)
	update = make(chan int)

	loadConfig()

	db := mysql.New("tcp", "", "212.193.32.4:3306", dataAsses.User, dataAsses.Password, "netmap")
	err := db.Connect()
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		return
	} else {
		fmt.Printf("Connected to database\n")
	}

	HostBase, _, err := db.Query("SELECT `ip` FROM `unetmap_host` WHERE `type_id`=4 AND `noconf`=0 AND `disabled`=0 AND ip <> 3232263980 ORDER BY ip ASC")
	if err != nil {
		fmt.Printf("Error select from database: %v\n", err)
	}
	num := len(HostBase)
	defer db.Close()

	for _, ht := range HostBase {
		go func(ip string) {
			err := getdata(ip)
			if err != nil {
				log.Println(ip, " : ", err)
				num--
				update <- num
				return
			}
			return
		}(ipDecode(ht.Uint(0)))
	}

	waiter(num)
}
