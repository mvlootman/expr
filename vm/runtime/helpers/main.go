package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"os"
	"strings"
	"text/template"
)

func main() {
	var b bytes.Buffer
	err := template.Must(
		template.New("helpers").
			Funcs(
				template.FuncMap{
					"cases":          func(op string) string { return cases(op, uints, ints, floats, decimal) },
					"cases_int_only": func(op string) string { return cases(op, uints, ints, decimal) },
					"cases_with_duration": func(op string) string {
						return cases(op, uints, ints, floats, decimal, []string{"time.Duration"})
					},
					"array_equal_cases": func() string {
						return arrayEqualCases(
							[]string{"string"}, uints, ints, floats, decimal,
						)
					},
				},
			).
			Parse(helpers),
	).Execute(&b, nil)
	if err != nil {
		panic(err)
	}

	formatted, err := format.Source(b.Bytes())
	if err != nil {
		// debugging
		fmt.Print(string(b.Bytes()))
		panic(err)
	}
	fmt.Print(string(formatted))
}

var ints = []string{
	"int",
	"int8",
	"int16",
	"int32",
	"int64",
}

var uints = []string{
	"uint",
	"uint8",
	"uint16",
	"uint32",
	"uint64",
}

var floats = []string{
	"float32",
	"float64",
}

var decimal = []string{
	"decimal.Decimal",
}

var decOps = map[string]string{
	"==": "Equal",
	"<":  "LessThan",
	">":  "GreaterThan",
	"<=": "LessThanOrEqual",
	">=": "GreaterThanOrEqual",
	"+":  "Add",
	"-":  "Sub",
	"/":  "Div",
	"*":  "Mul",
	"%":  "Mod",
}

func cases(op string, xs ...[]string) string {
	var types []string
	for _, x := range xs {
		types = append(types, x...)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Generating %s cases for %v\n", op, types)

	var out string
	echo := func(s string, xs ...any) {
		out += fmt.Sprintf(s, xs...) + "\n"
	}
	for _, a := range types {
		echo(`case %v:`, a)
		echo(`switch y := b.(type) {`)
		for _, b := range types {
			t := "int"
			if isDuration(a) || isDuration(b) {
				t = "time.Duration"
			}
			if isFloat(a) || isFloat(b) {
				t = "float64"
			}
			isDecimalA, isDecimalB := isDecimal(a), isDecimal(b)

			echo(`case %v:`, b)
			decOp, ok := decOps[op]
			if !ok {
				panic(fmt.Sprintf("key:%s not found in decimal operators map", op))
			}
			// We need to generate different case statements depending on whether:
			// both types are Decimal, the first type is Decimal, or the second type is Decimal
			if isDecimalA && isDecimalB {
				// both decimals we can perform decOp without conversion
				statement := fmt.Sprintf(`x.%s(y)`, decOp)
				if decOp == "Mod" {
					statement = convertDecimalToInt(statement)
				}
				echo("return " + statement)
			} else if isDecimalA {
				// x : Decimal, y : non-Decimal
				echo(caseDecimalDecAndNonDec(b, decOp, echo))
			} else if isDecimalB {
				// a : non-Decimal, b : Decimal
				echo(caseDecimalNonDecAndDec(a, decOp, echo))
			} else {
				// handle non-decimals
				if op == "/" {
					echo(`return float64(x) / float64(y)`)
				} else {
					echo(`return %v(x) %v %v(y)`, t, op, t)
				}
			}
		}
		echo(`}`)
	}
	return strings.TrimRight(out, "\n")
}

func convertDecimalToInt(statement string) string {
	return fmt.Sprintf("int(%s.IntPart())", statement)
}

func caseDecimalDecAndNonDec(b string, decOp string, echo func(s string, xs ...any)) string {
	// a: Decimal, b : non-Decimal
	// echo(`// debug caseDecimalDecAndNonDec: b:%s decOp:%s`, b, decOp)

	var statement string
	switch b {
	case "uint", "uint8", "uint16", "uint32":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromUint64(uint64(y)))`, decOp)
	case "uint64":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromUint64(y))`, decOp)
	case "int8", "int16":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromInt32(int32(y)))`, decOp)
	case "int32":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromInt32(y))`, decOp)
	case "int":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromInt(int64(y)))`, decOp)
	case "int64":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromInt(y))`, decOp)
	case "float32":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromFloat32(y))`, decOp)
	case "float64":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromFloat(y))`, decOp)
	case "time.Duration":
		statement = fmt.Sprintf(`x.%s(decimal.NewFromInt(int64(y)))`, decOp)
	default:
		fmt.Printf("decOp:%s type:%v\n", decOp, b)
		panic(errors.New("missing decimal datatype/operator"))
	}

	if decOp == "Mod" {
		statement = convertDecimalToInt(statement)
	}

	return "return " + statement
}

func caseDecimalNonDecAndDec(a string, decOp string, echo func(s string, xs ...any)) string {
	// a: non-Decimal, b : Decimal
	// echo(`// debug caseDecimalNonDecAndDec: a:%s decOp:%s`, a, decOp)

	var statement string

	switch a {
	case "uint", "uint8", "uint16", "uint32":
		statement = fmt.Sprintf(`decimal.NewFromUint64(uint64(x)).%s(y)`, decOp)
	case "uint64":
		statement = fmt.Sprintf(`decimal.NewFromUint64(x).%s(y)`, decOp)
	case "int8", "int16":
		statement = fmt.Sprintf(`decimal.NewFromInt32(int32(x)).%s(y)`, decOp)
	case "int32":
		statement = fmt.Sprintf(`decimal.NewFromInt32(x).%s(y)`, decOp)
	case "int":
		statement = fmt.Sprintf(`decimal.NewFromInt(int64(x)).%s(y)`, decOp)
	case "int64":
		statement = fmt.Sprintf(`decimal.NewFromInt(x).%s(y)`, decOp)
	case "float32":
		statement = fmt.Sprintf(`decimal.NewFromFloat32(x).%s(y)`, decOp)
	case "float64":
		statement = fmt.Sprintf(`decimal.NewFromFloat(x).%s(y)`, decOp)
	case "decimal.Decimal":
		statement = fmt.Sprintf(`decimal.NewFromFloat(x).%s(y)`, decOp)
	case "time.Duration":
		statement = fmt.Sprintf(`decimal.NewFromInt(int64(x)).%s(y)`, decOp)
	default:
		fmt.Printf("decOp:%s type:%v\n", decOp, a)
		panic(errors.New("missing decimal datatype/operator"))
	}

	if decOp == "Mod" {
		statement = convertDecimalToInt(statement)
	}

	return "return " + statement
}

func arrayEqualCases(xs ...[]string) string {
	var types []string
	for _, x := range xs {
		types = append(types, x...)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Generating array equal cases for %v\n", types)

	var out string
	echo := func(s string, xs ...any) {
		out += fmt.Sprintf(s, xs...) + "\n"
	}
	echo(`case []any:`)
	echo(`switch y := b.(type) {`)
	for _, a := range append(types, "any") {
		echo(`case []%v:`, a)
		echo(`if len(x) != len(y) { return false }`)
		echo(`for i := range x {`)
		echo(`if !Equal(x[i], y[i]) { return false }`)
		echo(`}`)
		echo("return true")
	}
	echo(`}`)
	for _, a := range types {
		echo(`case []%v:`, a)
		echo(`switch y := b.(type) {`)
		echo(`case []any:`)
		echo(`return Equal(y, x)`)
		echo(`case []%v:`, a)
		echo(`if len(x) != len(y) { return false }`)
		echo(`for i := range x {`)
		echo(`if x[i] != y[i] { return false }`)
		echo(`}`)
		echo("return true")
		echo(`}`)
	}
	return strings.TrimRight(out, "\n")
}

func isFloat(t string) bool {
	return strings.HasPrefix(t, "float")
}

func isDuration(t string) bool {
	return t == "time.Duration"
}

func isDecimal(t string) bool {
	return t == "decimal.Decimal"
}

const helpers = `// Code generated by vm/runtime/helpers/main.go. DO NOT EDIT.

package runtime

import (
	"fmt"
	"reflect"
	"time"

	"github.com/shopspring/decimal"
)

func Equal(a, b interface{}) bool {
	switch x := a.(type) {
	{{ cases "==" }}
	{{ array_equal_cases }}
	case string:
		switch y := b.(type) {
		case string:
			return x == y
		}
	case time.Time:
		switch y := b.(type) {
		case time.Time:
			return x.Equal(y)
		}
	case time.Duration:
		switch y := b.(type) {
		case time.Duration:
			return x == y
		}
	case bool:
		switch y := b.(type) {
		case bool:
			return x == y
		}
	}
	if IsNil(a) && IsNil(b) {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func Less(a, b interface{}) bool {
	switch x := a.(type) {
	{{ cases "<" }}
	case string:
		switch y := b.(type) {
		case string:
			return x < y
		}
	case time.Time:
		switch y := b.(type) {
		case time.Time:
			return x.Before(y)
		}
	case time.Duration:
		switch y := b.(type) {
		case time.Duration:
			return x < y
		}
	}
	panic(fmt.Sprintf("invalid operation: %T < %T", a, b))
}

func More(a, b interface{}) bool {
	switch x := a.(type) {
	{{ cases ">" }}
	case string:
		switch y := b.(type) {
		case string:
			return x > y
		}
	case time.Time:
		switch y := b.(type) {
		case time.Time:
			return x.After(y)
		}
	case time.Duration:
		switch y := b.(type) {
		case time.Duration:
			return x > y
		}
	}
	panic(fmt.Sprintf("invalid operation: %T > %T", a, b))
}

func LessOrEqual(a, b interface{}) bool {
	switch x := a.(type) {
	{{ cases "<=" }}
	case string:
		switch y := b.(type) {
		case string:
			return x <= y
		}
	case time.Time:
		switch y := b.(type) {
		case time.Time:
			return x.Before(y) || x.Equal(y)
		}
	case time.Duration:
		switch y := b.(type) {
		case time.Duration:
			return x <= y
		}
	}
	panic(fmt.Sprintf("invalid operation: %T <= %T", a, b))
}

func MoreOrEqual(a, b interface{}) bool {
	switch x := a.(type) {
	{{ cases ">=" }}
	case string:
		switch y := b.(type) {
		case string:
			return x >= y
		}
	case time.Time:
		switch y := b.(type) {
		case time.Time:
			return x.After(y) || x.Equal(y)
		}
	case time.Duration:
		switch y := b.(type) {
		case time.Duration:
			return x >= y
		}
	}
	panic(fmt.Sprintf("invalid operation: %T >= %T", a, b))
}

func Add(a, b interface{}) interface{} {
	switch x := a.(type) {
	{{ cases "+" }}
	case string:
		switch y := b.(type) {
		case string:
			return x + y
		}
	case time.Time:
		switch y := b.(type) {
		case time.Duration:
			return x.Add(y)
		}
	case time.Duration:
		switch y := b.(type) {
		case time.Time:
			return y.Add(x)
		case time.Duration:
			return x + y
		}
	}
	panic(fmt.Sprintf("invalid operation: %T + %T", a, b))
}

func Subtract(a, b interface{}) interface{} {
	switch x := a.(type) {
	{{ cases "-" }}
	case time.Time:
		switch y := b.(type) {
		case time.Time:
			return x.Sub(y)
		case time.Duration:
			return x.Add(-y)
		}
	case time.Duration:
		switch y := b.(type) {
		case time.Duration:
			return x - y
		}
	}
	panic(fmt.Sprintf("invalid operation: %T - %T", a, b))
}

func Multiply(a, b interface{}) interface{} {
	switch x := a.(type) {
	{{ cases_with_duration "*" }}
	}
	panic(fmt.Sprintf("invalid operation: %T * %T", a, b))
}

func Divide(a, b interface{}) interface{} {
	switch x := a.(type) {
	{{ cases "/" }}
	}
	panic(fmt.Sprintf("invalid operation: %T / %T", a, b))
}

func Modulo(a, b interface{}) int {
	switch x := a.(type) {
	{{ cases_int_only "%" }}
	}
	panic(fmt.Sprintf("invalid operation: %T %% %T", a, b))
}
`
