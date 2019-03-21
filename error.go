package tracerr

import (
	"fmt"
	"sort"
	"strings"
)

type Error interface {
	String() string
}

type Nil struct {
}

func (e *Nil) String() string {
	return "nil"
}

type Global struct {
	Name string
}

func (e *Global) String() string {
	return fmt.Sprintf("global(%s)", e.Name)
}

type MemoryAccess struct {
}

func (e *MemoryAccess) String() string {
	return "memory access"
}

type Unknown struct {
}

func (e *Unknown) String() string {
	return "Unknown"
}

type Modified struct {
	Closures []string
}

func (e *Modified) String() string {
	return fmt.Sprintf("modified by %v", e.Closures)
}

type Channel struct {
	Senders []Error
}

func (e *Channel) String() string {
	return fmt.Sprintf("channel %v", e.Senders)
}

type Pointer struct {
	Errors []Error
}

func (e *Pointer) String() string {
	return fmt.Sprintf("pointer %v", e.Errors)
}

type FunctionCall struct {
	Name  string
	Args  []string
	Index int
}

func (e *FunctionCall) String() string {
	return fmt.Sprintf("%s#%d(%s)", e.Name, e.Index, strings.Join(e.Args, ", "))
}

type Phi struct {
	Errors []Error
}

func (e *Phi) String() string {
	return fmt.Sprintf("%v", e.Errors)
}

type FunctionError struct {
	Name      string
	TupleSize int
	Errors    map[int][]Error
	HasError  map[int]map[string]bool
}

func (e *FunctionError) String() (ret string) {
	ret += fmt.Sprintf("func %s", e.Name)
	keys := make([]int, 0)
	for k, _ := range e.Errors {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		ret += fmt.Sprintf(" #%d: %v", k, e.Errors[k])
	}
	return
}

func (f *FunctionError) AddError(i int, e Error) {
	if _, ok := f.Errors[i]; !ok {
		f.Errors[i] = make([]Error, 0)
		f.HasError[i] = make(map[string]bool, 0)
	}

	switch err := e.(type) {
	case *FunctionCall:
		if f.Name == err.Name {
			return
		}
		if _, ok := f.HasError[i][err.String()]; ok {
			return
		}
		f.HasError[i][err.String()] = true

		f.Errors[i] = append(f.Errors[i], err)
	case *Pointer:
		for _, e := range err.Errors {
			f.AddError(i, e)
		}
	case *Phi:
		for _, e := range err.Errors {
			f.AddError(i, e)
		}
	case *Channel:
		for _, e := range err.Senders {
			f.AddError(i, e)
		}
	case *Modified:
		if len(err.Closures) == 0 {
			return
		}
		if _, ok := f.HasError[i][err.String()]; ok {
			return
		}
		f.HasError[i][err.String()] = true

		f.Errors[i] = append(f.Errors[i], err)
	case *MemoryAccess, *Global, *Unknown, *Nil:
		if _, ok := f.HasError[i][err.String()]; ok {
			return
		}
		f.HasError[i][err.String()] = true

		f.Errors[i] = append(f.Errors[i], err)
	}
}
