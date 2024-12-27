package strategy

import (
	"bytes"
	"founds/constant"
	"github.com/tidwall/gjson"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// https://data.howbuy.com/cgi/fund/v800z/zjzhchartdthc.json?zhid=67888190128&range=5N

var existFund = map[string]string{
	"161611": "融通内需驱动混合A",
	"001564": "东方红京东大数据混合A",
	"260112": "景顺长城能源基建混合A",
	"006624": "中泰玉衡价值优选混合A",
	"121010": "国投瑞银瑞源灵活配置混合A",
	"004475": "华泰柏瑞富利混合A",
	"090013": "大成竞争优势混合A",
	"004814": "中欧红利优享混合A",
}

func FundStrategy() []*constant.FundStrategy {
	url := "https://api.jiucaishuo.com/v2/fundchoose/result2"
	method := "POST"

	payload := []byte(`{
        "condition_id": "2199957"
    }`)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	if err != nil {
		log.Println(err)
		return nil
	}
	req.Header.Add("priority", "u=1, i")
	req.Header.Add("content-type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "api.jiucaishuo.com")
	req.Header.Add("Connection", "keep-alive")

	res, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return nil
	}
	response := string(responseBody)

	if err != nil || gjson.Get(response, "code").Int() != 0 {
		log.Println(response)
		return nil
	}

	fundList := gjson.Get(response, "data.position_table_data").Array()

	result := make([]*constant.FundStrategy, 0)

	cache := make(map[string]string)

	for _, data := range fundList {
		fundData := data.Map()
		item := &constant.FundStrategy{}
		fundName := fundData["name"].String()
		fundCode := fundData["code"].String()
		item.Name = fundName
		item.Code = fundCode
		fundInfo := fundData["list"].Array()
		item.PersonName = fundInfo[10].Map()["val"].String()
		item.PersonYear = fundInfo[3].Map()["val"].String()
		year5Sharpe, _ := strconv.Atoi(strings.Split(fundInfo[5].Map()["val"].String(), "/")[0])
		item.Year5Sharpe = year5Sharpe
		item.Gm = fundInfo[1].Map()["val"].String()
		item.YearTodayIncome = fundInfo[9].Map()["val"].String()
		item = setRate(item)
		if item == nil {
			continue
		}
		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Year5Sharpe < result[j].Year5Sharpe
	})

	list := make([]*constant.FundStrategy, 0)

	// 去重
	for _, fund := range result {
		_, ok := cache[fund.PersonName]
		if ok {
			continue
		}
		cache[fund.PersonName] = "1"
		_, ok = existFund[fund.Code]
		if !ok {
			fund.Name = "**" + fund.Name
		}
		list = append(list, fund)
	}

	for existCode, existName := range existFund {
		isDelete := true
		for _, strategy := range list {
			if existCode == strategy.Code {
				isDelete = false
			}
		}
		if isDelete {
			deleteFund := &constant.FundStrategy{Code: existCode, Name: "xx" + existName}
			list = append(list, deleteFund)
		}
	}

	return list
}

func setRate(strategy *constant.FundStrategy) *constant.FundStrategy {
	url := "https://danjuanfunds.com/djapi/fund/" + strategy.Code

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println(err)
		return strategy
	}

	req.Header.Set("User-Agent", "Apifox/1.0.0 (https://apifox.com)")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Host", "danjuanfunds.com")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return strategy
	}
	defer resp.Body.Close()

	responseBody, _ := ioutil.ReadAll(resp.Body)

	response := string(responseBody)
	if err != nil || gjson.Get(response, "result_code").Int() != 0 {
		log.Println(response)
		return strategy
	}

	if gjson.Get(response, "data.declare_status").String() == "0" {
		log.Println(response)
		return nil
	}

	baseDataArr := gjson.Get(response, "data.fir_header_base_data").Array()

	for _, baseData := range baseDataArr {
		if baseData.Map()["data_name"].String() == "年化收益（近5年）" {
			strategy.Year5Income = baseData.Map()["data_value_str"].String()
			strategy.Year5IncomeNumber = baseData.Map()["data_value_number"].Num
		}
	}

	if strategy.Year5IncomeNumber < 10 {
		return nil
	}

	return strategy
}
