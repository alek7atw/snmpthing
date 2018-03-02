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
)

var file *xlsx.File
var sheet *xlsx.Sheet

type Switch struct {
	ProductNum string
	SerialNum  string
	Hostname   string
}

var streamOfSwChanel chan Switch
var update chan int

func ipDecode(ip uint) string {
	return fmt.Sprintf("%d.%d.%d.%d", (ip>>24)%0xFF, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
}

func wrRow(data Switch) {
	row := sheet.AddRow()
	row.WriteStruct(&data, -1)
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
	for {
		select {
		case sw = <-streamOfSwChanel:
			wrRow(sw)
			counter++
			if counter >= num {
				return
			}
		case num = <-update:
			if counter >= num {
				return
			}
		}
	}
}

func main() {
	var err error
	streamOfSwChanel = make(chan Switch)
	update = make(chan int)

	file = xlsx.NewFile()
	sheet, err = file.AddSheet("Sheet1")
	if err != nil {
		fmt.Printf(err.Error())
	}

	db := mysql.New("tcp", "", "212.193.32.4:3306", "develop", "b1gbr0ther", "netmap")
	err = db.Connect()
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
			/*if ip == "192.168.111.44" {
				log.Println(ip, " : ","Slow as fuck")
				wgWriters.Done()
				return
			}*/
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
	fmt.Println("done")

	waiter(num)

	err = file.Save("switches.xlsx")
	if err != nil {
		fmt.Printf(err.Error())
	}
}
