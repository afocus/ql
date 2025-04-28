package ql

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var operation = map[string]string{
	"eq":   "=",
	"neq":  "<>",
	"gt":   ">",
	"ge":   ">=",
	"lt":   "<",
	"le":   "<=",
	"like": "like",
}

var SkipFieldErr = errors.New("skip field")

type CheckFun func(key, operation, v *string) error

// 匹配格式为  name:op(value),name:op(value),...
var pattern = regexp.MustCompile(`(?m)([\w\+]+):([\w]+)\((.*?)\),*`)

func ConvInterface(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("缺少必要的值")
	}
	if s[0] == '\'' || s[0] == '"' {
		if s[len(s)-1] != s[0] {
			return nil, errors.New("字符串结尾非法")
		}
		return s[1 : len(s)-1], nil
	}
	if s[0] >= '0' && s[0] <= '9' {
		if strings.Contains(s, ".") {
			return strconv.ParseFloat(s, 64)
		}
		return strconv.ParseInt(s, 10, 64)
	}
	return s, nil
}

func ConvInterfaces(sli string) ([]interface{}, error) {
	str := strings.Split(sli, ",")
	list := make([]interface{}, len(str))
	for i, a := range str {
		v, err := ConvInterface(a)
		if err != nil {
			return nil, err
		}
		list[i] = v
	}
	return list, nil
}

func buildqutos(n int) string {
	s := strings.Repeat("?", n)
	return strings.Join(strings.Split(s, ""), ",")
}

func ParsePart(key, op, value string) (q string, args []interface{}, err error) {
	key = fmt.Sprintf("`%s`", key)
	if ps := strings.Split(key, "+"); len(ps) > 1 {
		key = fmt.Sprintf("concat(%s)", strings.Join(ps, "`,`"))
	}
	switch op {
	case "in":
		args, err = ConvInterfaces(value)
		if err != nil {
			return
		}
		q = fmt.Sprintf("%s in (%s)", key, buildqutos(len(args)))
	case "range":
		args, err = ConvInterfaces(value)
		if err != nil {
			return
		}
		if len(args) != 2 {
			err = fmt.Errorf("%s range 必须两个值", key)
			return
		}
		q = fmt.Sprintf("%s between ? and ?", key)
	case "json":
		// charge_items:json('$[*].charger','0001') 修改为
		// charge_items->'$[*].charger'='0001' 弃用
		// json_contains(`charge_items`->'$[*].charger','0001','$')
		args, err = ConvInterfaces(value)
		if err != nil {
			return
		}
		if len(args) != 2 {
			err = fmt.Errorf("%s json 必须两个值", key)
			return
		}
		x, ok := args[0].(string)
		if !ok {
			err = errors.New("json 语法错误")
			return
		}
		q = fmt.Sprintf("json_contains(%s->'%s',?,'$')", key, x)
		args = []interface{}{fmt.Sprintf("%v", args[1])}
	case "json_extract":
		// field:json_extract('path',op,val...)
		// json_extract(field,path) op val
		var list []interface{}
		list, err = ConvInterfaces(value)
		if err != nil {
			return
		}
		if len(list) < 3 {
			err = fmt.Errorf("%s json_extract 必须至少3个值", key)
			return
		}
		x, ok := list[0].(string)
		if !ok {
			err = errors.New("json 语法错误")
			return
		}
		key = fmt.Sprintf("json_extract(%s,'%s')", key, x)
		op = list[1].(string)
		args = list[2:]
		switch op {
		case "in":
			q = fmt.Sprintf("%s in (%s)", key, buildqutos(len(args)))
		default:
			newop, ok := operation[op]
			if !ok {
				newop = op
			}
			q = fmt.Sprintf("%s %s ?", key, newop)
		}
	case "json_path":
		// $k:json_path($v,0) -> json_contains_path($k,'all',$v) = 0
		args, err = ConvInterfaces(value)
		if err != nil {
			return
		}
		if len(args) != 2 {
			err = fmt.Errorf("%s json_path 必须两个值", key)
			return
		}
		x, ok := args[0].(string)
		if !ok {
			err = errors.New("json_path 语法错误")
			return
		}
		q = fmt.Sprintf("json_contains_path(%s,'all','%s') = ?", key, x)
		args = args[1:]
	case "json_in":
		args, err = ConvInterfaces(value)
		if err != nil {
			return
		}
		if len(args) < 2 {
			err = fmt.Errorf("%s json_in 缺少参数", key)
			return
		}
		x, ok := args[0].(string)
		if !ok {
			err = errors.New("json_path 语法错误")
			return
		}
		args = args[1:]
		q = fmt.Sprintf("json_contains(%s->'%s',json_array(%s))", key, x, buildqutos(len(args)))
	default:
		newop, ok := operation[op]
		if !ok {
			err = fmt.Errorf("%s 无法识别操作符 %s", key, op)
			return
		}
		var v interface{}
		if v, err = ConvInterface(value); err != nil {
			return
		}
		q = fmt.Sprintf("%s %s ?", key, newop)
		args = []interface{}{v}
	}
	return
}

func Parse(content string, check CheckFun) (query string, args []interface{}, err error) {
	parts := make([]string, 0, 1)
	submatchs := pattern.FindAllStringSubmatch(content, -1)
	for _, part := range submatchs {
		match, key, op, val := part[0], part[1], part[2], part[3]
		if check != nil {
			if e := check(&key, &op, &val); e != nil {
				if errors.Is(e, SkipFieldErr) {
					continue
				} else {
					err = fmt.Errorf("%s %v", match, e)
					return
				}
			}
		}
		var partQuery string
		var partArgs []interface{}
		partQuery, partArgs, err = ParsePart(key, op, val)
		if err != nil {
			return
		}
		parts = append(parts, partQuery)
		args = append(args, partArgs...)
	}
	query = strings.Join(parts, " and ")
	return
}
