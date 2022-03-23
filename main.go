package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"sort"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"

	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func main() {
	flag.Parse()

	exercise8()
}

func messageBytes() []byte {
	foo := &Foo{Foo: "foo", Bar: &Bar{Bar: "bar"}}
	// any, err := anypb.New(foo)
	// if err != nil {
	// 	panic(err)
	// }
	b, err := proto.Marshal(foo)
	if err != nil {
		panic(err)
	}
	return b
}

func descBytes() []byte {
	set := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			// need both foo & bar: Foo depends on Bar
			protodesc.ToFileDescriptorProto(File_foo_proto),
			protodesc.ToFileDescriptorProto(File_bar_proto),
		},
	}
	b, err := proto.Marshal(set)
	if err != nil {
		panic(err)
	}
	return b
}

func exercise8() {
	fmt.Println("=== Exercise 8: evaluate proto from the wire ===")

	fileSet := &descriptorpb.FileDescriptorSet{}
	err := proto.Unmarshal(descBytes(), fileSet)
	if err != nil {
		panic(err)
	}

	// main.Foo is known to the caller
	fooFullName := "main.Foo"
	/**
	 * the following can be used to validate if fileSet is sufficient to
	 * resolve message descriptor
	 */
	reg, err := protodesc.NewFiles(fileSet)
	if err != nil {
		panic(err)
	}
	desc, err := reg.FindDescriptorByName(protoreflect.FullName(fooFullName))
	if err != nil {
		panic(err)
	}

	// typ := dynamicpb.NewMessageType(desc.(protoreflect.MessageDescriptor))
	// msg := typ.New()
	msg := dynamicpb.NewMessage(desc.(protoreflect.MessageDescriptor))

	err = proto.Unmarshal(messageBytes(), msg.Interface())
	if err != nil {
		panic(err)
	}
	// m, err := anypb.UnmarshalNew(msg, proto.UnmarshalOptions{})
	// if err != nil {
	// 	panic(err)
	// }
	// Declare the `x` and 'y' variables as input into the expression.
	env, _ := cel.NewEnv(
		// cel.TypeDescs(reg),
		cel.TypeDescs(fileSet),
		cel.Declarations(decls.NewVar("x", decls.NewObjectType(fooFullName))),
	)
	ast, iss := env.Compile(`x.bar`)
	if iss.Err() != nil {
		glog.Exit(iss.Err())
	}
	// Turn on optimization.
	vars := map[string]interface{}{"x": msg}
	program, _ := env.Program(ast, cel.EvalOptions(cel.OptExhaustiveEval))
	// Try benchmarking this evaluation with the optimization flag on and off.
	out, _, _ := eval(program, vars)
	b, err := proto.Marshal(out.Value().(proto.Message))
	if err != nil {
		panic(err)
	}
	var bar Bar
	proto.Unmarshal(b, &bar)
	spew.Dump(&bar)
	// 	fmt.Println()
}

// Functions to assist with CEL execution.

// eval will evaluate a given program `prg` against a set of variables `vars`
// and return the output, eval details (optional), or error that results from
// evaluation.
func eval(prg cel.Program,
	vars interface{},
) (out ref.Val, det *cel.EvalDetails, err error) {
	varMap, isMap := vars.(map[string]interface{})
	fmt.Println("------ input ------")
	if !isMap {
		fmt.Printf("(%T)\n", vars)
	} else {
		for k, v := range varMap {
			switch val := v.(type) {
			case proto.Message:
				bytes, err := prototext.Marshal(val)
				if err != nil {
					glog.Exitf("failed to marshal proto to text: %v", val)
				}
				fmt.Printf("%s = %s", k, string(bytes))
			case map[string]interface{}:
				b, _ := json.MarshalIndent(v, "", "  ")
				fmt.Printf("%s = %v\n", k, string(b))
			case uint64:
				fmt.Printf("%s = %vu\n", k, v)
			default:
				fmt.Printf("%s = %v\n", k, v)
			}
		}
	}
	fmt.Println()
	out, det, err = prg.Eval(vars)
	report(out, det, err)
	fmt.Println()
	return
}

// report prints out the result of evaluation in human-friendly terms.
func report(result ref.Val, details *cel.EvalDetails, err error) {
	fmt.Println("------ result ------")
	if err != nil {
		fmt.Printf("error: %s\n", err)
	} else {
		fmt.Printf("value: %v (%T)\n", result, result)
	}
	if details != nil {
		fmt.Printf("\n------ eval states ------\n")
		state := details.State()
		stateIDs := state.IDs()
		ids := make([]int, len(stateIDs))
		for i, id := range stateIDs {
			ids[i] = int(id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			v, found := state.Value(int64(id))
			if !found {
				continue
			}
			fmt.Printf("%d: %v (%T)\n", id, v, v)
		}
	}
}
