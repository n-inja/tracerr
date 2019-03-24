package tracerr

import (
	"go/token"
	"go/types"
	"log"
	"reflect"
	"strings"
	"sync"

	"golang.org/x/tools/go/ssa"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

var Analyzer = &analysis.Analyzer{
	Name: "tracerr",
	Doc:  Doc,
	Run:  run,
	Requires: []*analysis.Analyzer{
		buildssa.Analyzer,
	},
}
var errType *types.Interface
var once sync.Once
var chanPointerMap sync.Map
var closureMap sync.Map
var errMap sync.Map

// Doc is ...
const Doc = `tracerr is analyzing function which returns error in package`

func getError(fset *token.FileSet, pkgName string, v ssa.Value) (ret Error) {
	pos := fset.Position(v.Pos())

	if pos.IsValid() {
		if err, ok := errMap.Load(pos.String()); ok {
			return err.(Error)
		}
		ret = &Unknown{}
		errMap.Store(pos.String(), ret)
		defer func() {
			errMap.Store(pos.String(), ret)
		}()
	}

	getValue := func(v ssa.Value) string {
		if fn, ok := v.(*ssa.Function); ok {
			return fn.String()
		}
		if cnst, ok := v.(*ssa.Const); ok {
			if cnst.Value == nil {
				return "nil"
			} else {
				return cnst.Value.String()
			}
		}
		return v.Name()
	}

	funcCall := func(fn *ssa.Call, index int) Error {
		common := fn.Common()
		pos := fset.Position(fn.Pos())
		err := FunctionCall{}
		err.Name = pos.String()
		err.Index = index
		for i, arg := range common.Operands(nil) {
			if i > 0 {
				if types.Implements((*arg).Type(), errType) {
					err.Args = append(err.Args, getError(fset, pkgName, *arg).String())
				} else {
					err.Args = append(err.Args, getValue(*arg))
				}
			} else {
				err.Name = strings.TrimPrefix(getValue(*arg), pkgName)
			}
		}
		return &err
	}

	getChanPointerError := func(v ssa.Value) (ret []Error) {
		pos := fset.Position(v.Pos())
		if pos.IsValid() {
			if err, ok := chanPointerMap.Load(pos); ok {
				return err.([]Error)
			}
			defer func() {
				chanPointerMap.Store(pos, ret)
			}()
		}
		errs := make([]Error, 0)
		for _, ref := range *v.Referrers() {
			if _, ok := ref.(*ssa.UnOp); !ok {
				continue
			}

			for _, r := range *(ref.(*ssa.UnOp).Referrers()) {
				if _, ok := r.(*ssa.Send); !ok {
					continue
				}
				sender := r.(*ssa.Send).Operands(nil)[1]
				errs = append(errs, getError(fset, pkgName, *sender))
			}
		}
		ret = errs
		return
	}

	if !types.Implements(v.Type(), errType) {
		ret = nil
		return
	}
	switch val := v.(type) {
	case *ssa.UnOp:
		if val.Op == token.MUL {
			// deref error
			// normal assign / closure / array / slice

			pointer := *val.Operands(nil)[0]
			err := Pointer{}
			refs := pointer.Referrers()
			if g, ok := pointer.(*ssa.Global); ok {
				ret = &Global{Name: g.Name()}
				return
			}
			if refs == nil {
				ret = &Unknown{}
				return
			}
			for _, ref := range *refs {
				if closure, ok := ref.(*ssa.MakeClosure); ok {
					p := pointer.(*ssa.Alloc)
					makeclosure := (*closure.Operands(nil)[0]).(*ssa.Function)
					for _, freev := range makeclosure.FreeVars {
						if p.Comment != freev.Name() {
							continue
						}
						for _, ref := range *freev.Referrers() {
							if _, ok := ref.(*ssa.Store); !ok {
								continue
							}
							err.Errors = append(err.Errors, getError(fset, pkgName, *ref.Operands(nil)[1]))
						}
					}
				} else if store, ok := ref.(*ssa.Store); ok {
					err.Errors = append(err.Errors, getError(fset, pkgName, *store.Operands(nil)[1]))
				} else {
					if unop, ok := ref.(*ssa.UnOp); ok && unop.Op == token.MUL {
						if _, ok := (*unop.Operands(nil)[0]).(*ssa.IndexAddr); ok {
							ret = &MemoryAccess{}
							return
						} else if freevar, ok := (*unop.Operands(nil)[0]).(*ssa.FreeVar); ok {
							pos := -1
							for i, v := range freevar.Parent().FreeVars {
								if freevar.Name() == v.Name() {
									pos = i
								}
							}
							interf, ok := closureMap.Load(freevar.Parent().Name())
							if pos < 0 || !ok {
								return &Unknown{}
							}

							closures := interf.([]*ssa.Value)

							for _, ref := range *(*closures[pos + 1]).Referrers() {
								if store, ok := ref.(*ssa.Store); ok {
									err.Errors = append(err.Errors, getError(fset, pkgName, *store.Operands(nil)[1]))
								}
							}
						}
					}
				}
			}
			ret = &err
			return
		} else if val.Op == token.ARROW {
			// channel
			ch := (*val.Operands(nil)[0])

			// go rutine / anonymouse function
			if pointer, ok := ch.(*ssa.UnOp); ok && pointer.Op == token.MUL {
				channel := *pointer.Operands(nil)[0]
				err := Channel{}

				if _, ok := channel.(*ssa.Alloc); !ok {
					log.Println("unreachable", val.Pos(), reflect.TypeOf(val), val)
					ret = &Unknown{}
					return
				}

				alloc := channel.(*ssa.Alloc)
				refs := alloc.Referrers()
				mod := Modified{}

				if refs == nil {
					ret = &err
					return
				}
				for _, ref := range *refs {
					switch t := ref.(type) {
					case *ssa.MakeClosure:
						mod.Closures = append(mod.Closures, strings.TrimPrefix((*t.Operands(nil)[0]).String(), pkgName))
					case *ssa.UnOp:
						// DO NOTHING
					case *ssa.Store:
						senders := getChanPointerError(*t.Operands(nil)[0])
						for _, s := range senders {
							err.Senders = append(err.Senders, s)
						}
					default:
						log.Println(reflect.TypeOf(t), t)
						ret = &Unknown{}
						return
					}
				}
				err.Senders = append(err.Senders, &mod)
				ret = &err
				return
			}

			// only function
			if channel, ok := ch.(*ssa.MakeChan); ok {
				refs := channel.Referrers()
				err := Channel{}
				if refs == nil {
					ret = &err
					return
				}
				for _, ref := range *refs {
					if send, ok := ref.(*ssa.Send); ok {
						err.Senders = append(err.Senders, getError(fset, pkgName, *send.Operands(nil)[1]))
					}
				}
				ret = &err
				return
			}
		}
		log.Println("unreachable", val.Op)

	case *ssa.Extract:
		if fn, ok := val.Tuple.(*ssa.Call); ok {
			ret = funcCall(fn, val.Index)
			return
		}
		if _, ok := val.Tuple.(*ssa.TypeAssert); ok {
			// type assert
			ret = &Unknown{}
			return
		}
		log.Println("unreachable", reflect.TypeOf(val.Tuple), val.Tuple)

	case *ssa.Call:
		ret = funcCall(val, 0)
		return

	case *ssa.Const:
		if val.IsNil() {
			ret = &Nil{}
			return
		}
	case *ssa.Phi:
		err := Phi{}
		for _, op := range val.Operands(nil) {
			err.Errors = append(err.Errors, getError(fset, pkgName, *op))
		}
		ret = &err
		return
	case *ssa.Lookup:
		ret = &MemoryAccess{}
		return
	default:
		ret = &Unknown{}
		return
	}

	return
}

func run(pass *analysis.Pass) (interface{}, error) {

	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	pkgName := strings.TrimPrefix(ssaInput.Pkg.String(), "package ") + "."

	errType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

	isTest := func(f *ssa.Function) bool {
		return strings.HasSuffix(pass.Fset.Position(f.Pos()).Filename, "_test.go")
	}

	traceFunc := func(f *ssa.Function) *FunctionError {
		ferr := &FunctionError{}
		ferr.Errors = make(map[int][]Error, 0)
		ferr.HasError = make(map[int]map[string]bool, 0)
		for _, b := range f.DomPreorder() {
			for _, inst := range b.Instrs {
				ops := inst.Operands(nil)
				vs := make([]ssa.Value, 0)
				for _, v := range ops {
					vs = append(vs, *v)
				}

				if makeClosure, ok := inst.(*ssa.MakeClosure); ok {
					closureMap.Store((*makeClosure.Operands(nil)[0]).(*ssa.Function).Name(), makeClosure.Operands(nil))
				}

				if rets, ok := inst.(*ssa.Return); ok && rets.Pos().IsValid() {
					ferr.TupleSize = len(rets.Operands(nil))
					for i, vp := range rets.Operands(nil) {
						if types.Implements((*vp).Type(), errType) {
							ferr.AddError(i, getError(pass.Fset, pkgName, *vp))
						}
					}
				}
			}
		}
		return ferr
	}

	for _, fn := range ssaInput.SrcFuncs {

		e := traceFunc(fn)

		if len(e.Errors) == 0 {
			continue
		}

		if isTest(fn) {
			continue
		}

		e.Name = strings.TrimPrefix(fn.String(), pkgName)
		pass.Reportf(fn.Pos(), "%s", e.String())
	}

	return nil, nil
}
