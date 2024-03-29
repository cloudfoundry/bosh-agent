package action

import (
	"bytes"
	"encoding/json"
	"reflect"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type Runner interface {
	Run(action Action, payload []byte, protocolVersion ProtocolVersion) (value interface{}, err error)
	Resume(action Action, payload []byte) (value interface{}, err error)
}

func NewRunner() Runner {
	return concreteRunner{}
}

type concreteRunner struct{}

func (r concreteRunner) Run(action Action, payloadBytes []byte, protocolVersion ProtocolVersion) (value interface{}, err error) {
	payloadArgs, err := r.extractJSONArguments(payloadBytes)
	if err != nil {
		err = bosherr.WrapError(err, "Extracting json arguments")
		return
	}

	actionValue := reflect.ValueOf(action)
	runMethodValue := actionValue.MethodByName("Run")
	if runMethodValue.Kind() != reflect.Func {
		err = bosherr.Error("Run method not found")
		return
	}

	runMethodType := runMethodValue.Type()
	if r.invalidReturnTypes(runMethodType) {
		err = bosherr.Error("Run method should return a value and an error")
		return
	}

	methodArgs, err := r.extractMethodArgs(runMethodType, protocolVersion, payloadArgs)
	if err != nil {
		err = bosherr.WrapError(err, "Extracting method arguments from payload")
		return
	}

	values := runMethodValue.Call(methodArgs)
	return r.extractReturns(values)
}

func (r concreteRunner) Resume(action Action, payloadBytes []byte) (value interface{}, err error) {
	return action.Resume()
}

func (r concreteRunner) extractJSONArguments(payloadBytes []byte) (args []interface{}, err error) {
	type payloadType struct {
		Arguments []interface{} `json:"arguments"`
	}
	payload := payloadType{}

	decoder := json.NewDecoder(bytes.NewReader(payloadBytes))
	decoder.UseNumber()
	err = decoder.Decode(&payload)
	if err != nil {
		err = bosherr.WrapError(err, "Unmarshalling payload arguments to interface{} types")
	}
	args = payload.Arguments
	return
}

func (r concreteRunner) invalidReturnTypes(methodType reflect.Type) (valid bool) {
	if methodType.NumOut() != 2 {
		return true
	}

	secondReturnType := methodType.Out(1)
	if secondReturnType.Kind() != reflect.Interface {
		return true
	}

	errorType := reflect.TypeOf(bosherr.Error(""))
	secondReturnIsError := errorType.Implements(secondReturnType)
	if !secondReturnIsError {
		return true
	}

	return
}

func (r concreteRunner) extractMethodArgs(runMethodType reflect.Type, protocolVersion ProtocolVersion, args []interface{}) ([]reflect.Value, error) {
	methodArgs := []reflect.Value{}
	numberOfArgs := runMethodType.NumIn()
	numberOfReqArgs := numberOfArgs

	if runMethodType.IsVariadic() {
		numberOfReqArgs--
	}

	argsOffset := 0

	if numberOfArgs > 0 {
		firstArgType := runMethodType.In(0)

		if firstArgType.Name() == "ProtocolVersion" {
			methodArgs = append(methodArgs, reflect.ValueOf(protocolVersion))
			numberOfReqArgs--
			argsOffset++
		}
	}

	if len(args) < numberOfReqArgs {
		return methodArgs, bosherr.Errorf("Not enough arguments, expected %d, got %d", numberOfReqArgs, len(args))
	}

	for i, argFromPayload := range args {
		var rawArgBytes []byte
		rawArgBytes, err := json.Marshal(argFromPayload)
		if err != nil {
			return methodArgs, bosherr.WrapError(err, "Marshalling action argument")
		}

		argType, typeFound := r.getMethodArgType(runMethodType, i+argsOffset)
		if !typeFound {
			continue
		}

		argValuePtr := reflect.New(argType)

		err = json.Unmarshal(rawArgBytes, argValuePtr.Interface())
		if err != nil {
			return methodArgs, bosherr.WrapError(err, "Unmarshalling action argument")
		}

		methodArgs = append(methodArgs, reflect.Indirect(argValuePtr))
	}

	return methodArgs, nil
}

func (r concreteRunner) getMethodArgType(methodType reflect.Type, index int) (argType reflect.Type, found bool) {
	numberOfArgs := methodType.NumIn()

	switch {
	case !methodType.IsVariadic() && index >= numberOfArgs:
		return nil, false

	case methodType.IsVariadic() && index >= numberOfArgs-1:
		sliceType := methodType.In(numberOfArgs - 1)
		return sliceType.Elem(), true

	default:
		return methodType.In(index), true
	}
}

func (r concreteRunner) extractReturns(values []reflect.Value) (value interface{}, err error) {
	errValue := values[1]
	if !errValue.IsNil() {
		errorValues := errValue.MethodByName("Error").Call([]reflect.Value{})
		err = bosherr.Error(errorValues[0].String())
	}

	value = values[0].Interface()
	return
}
