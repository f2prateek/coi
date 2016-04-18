package dagger

import (
	"fmt"
	"reflect"

	"github.com/facebookgo/structtag"
)

// ObjectGraph is a graph of objects linked by their dependencies.
type ObjectGraph interface {
	// Inject the fields of `target`.
	Inject(target interface{})
	// Get retrieves an instance of the given type.
	Get(reflect.Type) interface{}
}

// NewObjectGraph returns a new dependency graph using the given modules.
func NewObjectGraph(modules ...Module) ObjectGraph {
	graph := make(map[reflect.Type]*moduleProvider)
	for _, module := range modules {
		moduleType := reflect.TypeOf(module)
		moduleValue := reflect.ValueOf(module)
		for i := 0; i < moduleType.NumMethod(); i++ {
			method := moduleType.Method(i)
			if !isMethodExported(method) {
				// todo: log
				continue
			}
			if !isProviderMethod(method) {
				// todo: log
				continue
			}

			if method.Type.NumOut() != 1 {
				panic(fmt.Errorf("Provider methods can return only one argument. %s returns %d values.", method.Type, method.Type.NumOut()))
			}
			binding := method.Type.Out(0)

			var dependencies []reflect.Type
			for j := 1; j < method.Type.NumIn(); j++ {
				dependencies = append(dependencies, method.Type.In(j))
			}

			graph[binding] = &moduleProvider{moduleValue, dependencies, method}
		}
	}
	return &objectGraph{graph}
}

type moduleProvider struct {
	moduleValue    reflect.Value
	dependencies   []reflect.Type
	providerMethod reflect.Method
}

func (p *moduleProvider) get(o ObjectGraph) interface{} {
	args := make([]reflect.Value, 0, 1+len(p.dependencies))
	args = append(args, p.moduleValue)
	for _, d := range p.dependencies {
		args = append(args, reflect.ValueOf(o.Get(d)))
	}
	return p.providerMethod.Func.Call(args)[0].Interface()
}

type objectGraph struct {
	graph map[reflect.Type]*moduleProvider
}

func (o *objectGraph) Inject(ptr interface{}) {
	ptrType := reflect.TypeOf(ptr)
	if ptrType.Kind() != reflect.Ptr {
		panic("can only inject into pointers")
	}

	v := reflect.ValueOf(ptr).Elem()
	if v.Kind() != reflect.Struct {
		panic("must provide a pointer to struct")
	}
	t := reflect.TypeOf(v.Interface())

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		ok, _, err := structtag.Extract("inject", string(field.Tag))
		if err != nil {
			continue
		}
		if !ok {
			continue
		}

		val := o.Get(field.Type)
		fieldValue := v.Field(i)
		fieldValue.Set(reflect.ValueOf(val))
	}
}

func (o *objectGraph) Get(t reflect.Type) interface{} {
	return o.graph[t].get(o)
}
