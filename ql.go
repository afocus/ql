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
		return strconv.ParseFloat(s, 64)
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

func Parse(content string, check CheckFun) (query string, args []interface{}, err error) {
	parts := make([]string, 0, 1)
	submatchs := pattern.FindAllStringSubmatch(content, -1)
	for _, part := range submatchs {
		match, key, op, val := part[0], part[1], part[2], part[3]
		if check != nil {
			if e := check(&key, &op, &val); e != nil {
				err = fmt.Errorf("%s %v", match, e)
				return
			}
		}
		key = fmt.Sprintf("`%s`", key)
		if ps := strings.Split(key, "+"); len(ps) > 1 {
			key = fmt.Sprintf("concat(%s)", strings.Join(ps, "`,`"))
		}
		switch op {
		case "in":
			var list []interface{}
			list, err = ConvInterfaces(val)
			if err != nil {
				return
			}
			val = strings.Repeat("?, ", len(list))
			val = fmt.Sprintf("(%s)", val[:len(val)-2])
			args = append(args, list...)
		case "range":
			var list []interface{}
			list, err = ConvInterfaces(val)
			if err != nil {
				return
			}
			if len(list) != 2 {
				err = fmt.Errorf("%s range 必须两个值", match)
				return
			}
			op, val = "between", "? and ?"
			args = append(args, list...)
		case "json":
			// charge_items:json('$[*].charger','0001') 修改为
			// charge_items->'$[*].charger'='0001'
			var list []interface{}
			list, err = ConvInterfaces(val)
			if err != nil {
				return
			}
			if len(list) != 2 {
				err = fmt.Errorf("%s json 必须两个值", match)
				return
			}
			x, ok := list[0].(string)
			if !ok {
				err = errors.New("json 语法错误")
				return
			}
			key = fmt.Sprintf("JSON_CONTAINS(%s -> '%s', ?, '$')", key, x)
			op, val = "", ""
			args = append(args, fmt.Sprintf("%v", list[1]))
		case "json_extract":
			// field:json_extract('path',op,val...)
			// json_extract(field,path) op val
			var list []interface{}
			list, err = ConvInterfaces(val)
			if err != nil {
				return
			}
			if len(list) < 3 {
				err = fmt.Errorf("%s json_extract 必须至少3个值", match)
				return
			}
			x, ok := list[0].(string)
			if !ok {
				err = errors.New("json 语法错误")
				return
			}
			key = fmt.Sprintf("json_extract(%s , '%s')", key, x)
			op = list[1].(string)
			switch op {
			case "in":
				val = strings.Repeat("?, ", len(list)-2)
				val = fmt.Sprintf("(%s)", val[:len(val)-2])
				args = append(args, list[2:]...)
			default:
				newop, ok := operation[op]
				if ok {
					val, op = "?", newop
					args = append(args, list[2:]...)
				}
			}
		case "json_path":
			// $k:json_path($v,0) -> json_contains_path($k,'all',$v) = 0
			var list []interface{}
			list, err = ConvInterfaces(val)
			if err != nil {
				return
			}
			if len(list) != 2 {
				err = fmt.Errorf("%s json_path 必须两个值", match)
				return
			}
			x, ok := list[0].(string)
			if !ok {
				err = errors.New("json_path 语法错误")
				return
			}
			key = fmt.Sprintf("json_contains_path(%s,'all',?) = ?", key)
			op, val = "", ""
			args = append(args, x, list[1])
		case "json_in":
			var list []interface{}
			list, err = ConvInterfaces(val)
			if err != nil {
				return
			}
			if len(list) < 2 {
				err = fmt.Errorf("%s json_in 缺少参数", match)
				return
			}
			x, ok := list[0].(string)
			if !ok {
				err = errors.New("json_path 语法错误")
				return
			}
			key = fmt.Sprintf("JSON_CONTAINS(%s", key)
			op = fmt.Sprintf("->'%s',", x)
			val = strings.Repeat("?, ", len(list)-1)
			val = fmt.Sprintf("JSON_ARRAY(%s))", val[:len(val)-2])
			args = append(args, list[1:]...)
		default:
			newop, ok := operation[op]
			if !ok {
				err = fmt.Errorf("%s 无法识别操作符 %s", match, op)
				return
			}
			var v interface{}
			if v, err = ConvInterface(val); err != nil {
				return
			}
			op, val = newop, "?"
			args = append(args, v)
		}
		parts = append(parts, fmt.Sprintf("%s %s %s", key, op, val))
	}
	query = strings.Join(parts, " and ")
	return
}
