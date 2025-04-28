package ql

import (
	"fmt"
	"testing"
)

func TestXxx(t *testing.T) {
	testdata := []struct {
		Raw   string
		Query string
		Args  []interface{}
	}{
		{
			Raw:   "name:eq('abc')",
			Query: "`name` = ?",
			Args:  []interface{}{"abc"},
		},
		{
			Raw:   "name:in('aaa','bbb','ccc')",
			Query: "`name` in (?,?,?)",
			Args:  []interface{}{"aaa", "bbb", "ccc"},
		},
		{
			Raw:   "name:range(0,1000)",
			Query: "`name` between ? and ?",
			Args:  []interface{}{0, 1000},
		},
		{
			Raw:   "name:json('$[*].charger','0001')",
			Query: "json_contains(`name`->'$[*].charger',?,'$')",
			Args:  []interface{}{"0001"},
		},
		{
			Raw:   "name:json_extract('$.recover_status','in',1,2,3)",
			Query: "json_extract(`name`,'$.recover_status') in (?,?,?)",
			Args:  []interface{}{1, 2, 3},
		},
		{
			Raw:   "name:json_path('$.recover_status',0)",
			Query: "json_contains_path(`name`,'all','$.recover_status') = ?",
			Args:  []interface{}{0},
		},
		{
			Raw:   "name:json_in('$.recover_status',1,2)",
			Query: "json_contains(`name`->'$.recover_status',json_array(?,?))",
			Args:  []interface{}{1, 2},
		},
		{
			Raw:   "id:eq('aaaa'),name:like('%%foc'),age:range(16,32)",
			Query: "`id` = ? and `name` like ? and `age` between ? and ?",
			Args:  []interface{}{"aaaa", "%%foc", 16, 32},
		},
	}

	for _, v := range testdata {
		fmt.Println(v.Raw)
		q, args, err := Parse(v.Raw, nil)
		if err != nil {
			t.FailNow()
		}
		fmt.Println(q, args)
		fmt.Println(v.Query, v.Args)
		if q != v.Query {
			t.FailNow()
		}
	}
}
