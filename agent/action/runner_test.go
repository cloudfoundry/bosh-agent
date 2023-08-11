package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	fakeaction "github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	"github.com/stretchr/testify/assert"
)

type valueType struct {
	ID      int
	Success bool
}

type argsType struct {
	User     string `json:"user"`
	Password string `json:"pwd"`
	ID       int    `json:"id"`
}

type argumentWithTypes struct {
	IntType   int   `json:"int_type"`
	Int32Type int32 `json:"int32_type"`
	Int64Type int64 `json:"int64_type"`

	Float32Type float32 `json:"float32_type"`
	Float64Type float64 `json:"float64_type"`

	UintType uint `json:"uint_type"`

	StringType string `json:"string_type"`

	BoolType bool `json:"bool_type"`
}

type actionWithTypes struct {
	Value valueType
	Err   error
	Arg   argumentWithTypes
}

func (a *actionWithTypes) IsAsynchronous(_ action.ProtocolVersion) bool {
	return false
}

func (a *actionWithTypes) IsPersistent() bool {
	return false
}

func (a *actionWithTypes) IsLoggable() bool {
	return true
}

func (a *actionWithTypes) Run(arg argumentWithTypes) (valueType, error) {
	a.Arg = arg
	return a.Value, a.Err
}

func (a *actionWithTypes) Resume() (interface{}, error) {
	return nil, nil
}

func (a *actionWithTypes) Cancel() error {
	return nil
}

type actionWithSingleStringArgument struct {
	Value valueType
	Err   error

	Arg string
}

func (a *actionWithSingleStringArgument) IsAsynchronous(_ action.ProtocolVersion) bool {
	return false
}

func (a *actionWithSingleStringArgument) IsPersistent() bool {
	return false
}

func (a *actionWithSingleStringArgument) IsLoggable() bool {
	return true
}

func (a *actionWithSingleStringArgument) Run(arg string) (valueType, error) {
	a.Arg = arg
	return a.Value, a.Err
}

func (a *actionWithSingleStringArgument) Resume() (interface{}, error) {
	return nil, nil
}

func (a *actionWithSingleStringArgument) Cancel() error {
	return nil
}

type actionWithGoodRunMethod struct {
	Value valueType
	Err   error

	SubAction string
	SomeID    int
	ExtraArgs argsType
	SliceArgs []string
}

func (a *actionWithGoodRunMethod) IsAsynchronous(_ action.ProtocolVersion) bool {
	return false
}

func (a *actionWithGoodRunMethod) IsPersistent() bool {
	return false
}

func (a *actionWithGoodRunMethod) IsLoggable() bool {
	return true
}

func (a *actionWithGoodRunMethod) Run(subAction string, someID int, extraArgs argsType, sliceArgs []string) (valueType, error) {
	a.SubAction = subAction
	a.SomeID = someID
	a.ExtraArgs = extraArgs
	a.SliceArgs = sliceArgs
	return a.Value, a.Err
}

func (a *actionWithGoodRunMethod) Resume() (interface{}, error) {
	return nil, nil
}

func (a *actionWithGoodRunMethod) Cancel() error {
	return nil
}

type actionWithOptionalRunArgument struct {
	SubAction    string
	OptionalArgs []argsType

	Value valueType
	Err   error
}

func (a *actionWithOptionalRunArgument) IsAsynchronous(_ action.ProtocolVersion) bool {
	return false
}

func (a *actionWithOptionalRunArgument) IsPersistent() bool {
	return false
}

func (a *actionWithOptionalRunArgument) IsLoggable() bool {
	return true
}

func (a *actionWithOptionalRunArgument) Run(subAction string, optionalArgs ...argsType) (valueType, error) {
	a.SubAction = subAction
	a.OptionalArgs = optionalArgs
	return a.Value, a.Err
}

func (a *actionWithOptionalRunArgument) Resume() (interface{}, error) {
	return nil, nil
}

func (a *actionWithOptionalRunArgument) Cancel() error {
	return nil
}

type actionWithoutRunMethod struct{}

func (a *actionWithoutRunMethod) IsAsynchronous(_ action.ProtocolVersion) bool {
	return false
}

func (a *actionWithoutRunMethod) IsPersistent() bool {
	return false
}

func (a *actionWithoutRunMethod) IsLoggable() bool {
	return true
}

func (a *actionWithoutRunMethod) Resume() (interface{}, error) {
	return nil, nil
}

func (a *actionWithoutRunMethod) Cancel() error {
	return nil
}

type actionWithOneRunReturnValue struct{}

func (a *actionWithOneRunReturnValue) IsAsynchronous(_ action.ProtocolVersion) bool {
	return false
}

func (a *actionWithOneRunReturnValue) IsPersistent() bool {
	return false
}

func (a *actionWithOneRunReturnValue) IsLoggable() bool {
	return true
}

func (a *actionWithOneRunReturnValue) Run() error {
	return nil
}

func (a *actionWithOneRunReturnValue) Resume() (interface{}, error) {
	return nil, nil
}

func (a *actionWithOneRunReturnValue) Cancel() error {
	return nil
}

type actionWithSecondReturnValueNotError struct{}

func (a *actionWithSecondReturnValueNotError) IsAsynchronous(_ action.ProtocolVersion) bool {
	return false
}

func (a *actionWithSecondReturnValueNotError) IsPersistent() bool {
	return false
}

func (a *actionWithSecondReturnValueNotError) IsLoggable() bool {
	return true
}

func (a *actionWithSecondReturnValueNotError) Run() (interface{}, string) {
	return nil, ""
}

func (a *actionWithSecondReturnValueNotError) Resume() (interface{}, error) {
	return nil, nil
}

func (a *actionWithSecondReturnValueNotError) Cancel() error {
	return nil
}

type actionWithProtocolVersion struct {
	ProtocolVersion action.ProtocolVersion
	SubAction       string
}

func (a *actionWithProtocolVersion) IsAsynchronous(_ action.ProtocolVersion) bool {
	return false
}

func (a *actionWithProtocolVersion) IsPersistent() bool {
	return false
}

func (a *actionWithProtocolVersion) IsLoggable() bool {
	return true
}

func (a *actionWithProtocolVersion) Run(protocolVersion action.ProtocolVersion, subAction string) (valueType, error) {
	a.ProtocolVersion = protocolVersion
	a.SubAction = subAction

	return valueType{}, nil
}

func (a *actionWithProtocolVersion) Resume() (interface{}, error) {
	return nil, nil
}

func (a *actionWithProtocolVersion) Cancel() error {
	return nil
}

var _ = Describe("concreteRunner", func() {
	It("runner run parses the payload", func() {
		runner := action.NewRunner()

		expectedValue := valueType{ID: 13, Success: true}
		expectedErr := errors.New("fake-run-error")

		action := &actionWithGoodRunMethod{Value: expectedValue, Err: expectedErr}
		payload := `{
				"arguments":[
					"setup",
					 123,
					 {"user":"rob","pwd":"rob123","id":12},
					 ["a","b","c"],
					 456
				]
			}`

		value, err := runner.Run(action, []byte(payload), 0)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("fake-run-error"))

		Expect(value).To(Equal(expectedValue))
		Expect(err).To(Equal(expectedErr))

		Expect(action.SubAction).To(Equal("setup"))
		Expect(action.SomeID).To(Equal(123))
		Expect(action.ExtraArgs).To(Equal(argsType{User: "rob", Password: "rob123", ID: 12}))
		Expect(action.SliceArgs).To(Equal([]string{"a", "b", "c"}))
	})

	It("runner run errs when actions not enough arguments", func() {
		runner := action.NewRunner()

		expectedValue := valueType{ID: 13, Success: true}

		action := &actionWithGoodRunMethod{Value: expectedValue}
		payload := `{"arguments":["setup"]}`

		_, err := runner.Run(action, []byte(payload), 0)
		Expect(err).To(HaveOccurred())
	})

	It("runner runs successfully when action is passed more arguments than required", func() {
		runner := action.NewRunner()

		expectedValue := valueType{ID: 13, Success: true}

		action := &actionWithSingleStringArgument{Value: expectedValue}
		payload := `{"arguments":["setup", "additional extra argument", "another extra argument"]}`

		_, err := runner.Run(action, []byte(payload), 0)
		Expect(err).ToNot(HaveOccurred())
	})

	It("runner run errs when action arguments types do not match", func() {
		runner := action.NewRunner()

		expectedValue := valueType{ID: 13, Success: true}

		action := &actionWithGoodRunMethod{Value: expectedValue}
		payload := `{"arguments":[123, "setup", {"user":"rob","pwd":"rob123","id":12}]}`

		_, err := runner.Run(action, []byte(payload), 0)
		Expect(err).To(HaveOccurred())
	})

	It("extracts argument types correctly", func() {
		runner := action.NewRunner()

		actionWithTypes := &actionWithTypes{}

		payload := `{
				"arguments":[{
					"int_type":-1024000,
					"int32_type":-1024000,
					"int64_type":-1024000,
					"float32_type":1e2,
					"float64_type":1.024e+06,
					"uint_type":1024000,
					"string_type":"fake-string",
					"bool_type":false
				}]
			}`
		_, err := runner.Run(actionWithTypes, []byte(payload), 0)
		Expect(err).ToNot(HaveOccurred())

		Expect(actionWithTypes.Arg.IntType).To(Equal(int(-1024000)))
		Expect(actionWithTypes.Arg.Int32Type).To(Equal(int32(-1024000)))
		Expect(actionWithTypes.Arg.Int64Type).To(Equal(int64(-1024000)))
		Expect(actionWithTypes.Arg.Float32Type).To(Equal(float32(100)))
		Expect(actionWithTypes.Arg.Float64Type).To(Equal(float64(1.024e+06)))
		Expect(actionWithTypes.Arg.UintType).To(Equal(uint(1024000)))
		Expect(actionWithTypes.Arg.StringType).To(Equal("fake-string"))
		Expect(actionWithTypes.Arg.BoolType).To(Equal(false))
	})

	It("runner handles optional arguments being passed in", func() {
		runner := action.NewRunner()

		expectedValue := valueType{ID: 13, Success: true}
		expectedErr := errors.New("fake-run-error")

		actionWithOptionalRunArgument := &actionWithOptionalRunArgument{Value: expectedValue, Err: expectedErr}
		payload := `{"arguments":["setup", {"user":"rob","pwd":"rob123","id":12}, {"user":"bob","pwd":"bob123","id":13}]}`

		value, err := runner.Run(actionWithOptionalRunArgument, []byte(payload), 0)

		Expect(value).To(Equal(expectedValue))
		Expect(err).To(Equal(expectedErr))

		Expect(actionWithOptionalRunArgument.SubAction).To(Equal("setup"))
		assert.Equal(GinkgoT(), actionWithOptionalRunArgument.OptionalArgs, []argsType{
			{User: "rob", Password: "rob123", ID: 12},
			{User: "bob", Password: "bob123", ID: 13},
		})
	})

	It("runner handles optional arguments when not passed in", func() {
		runner := action.NewRunner()
		actionWithOptionalRunArgument := &actionWithOptionalRunArgument{}
		payload := `{"arguments":["setup"]}`

		_, err := runner.Run(actionWithOptionalRunArgument, []byte(payload), 0)
		Expect(err).NotTo(HaveOccurred())

		Expect(actionWithOptionalRunArgument.SubAction).To(Equal("setup"))
		Expect(actionWithOptionalRunArgument.OptionalArgs).To(Equal([]argsType{}))
	})

	It("runner run errs when action does not implement run", func() {
		runner := action.NewRunner()
		_, err := runner.Run(&actionWithoutRunMethod{}, []byte(`{"arguments":[]}`), 0)
		Expect(err).To(HaveOccurred())
	})

	It("runner run errs when actions run does not return two values", func() {
		runner := action.NewRunner()
		_, err := runner.Run(&actionWithOneRunReturnValue{}, []byte(`{"arguments":[]}`), 0)
		Expect(err).To(HaveOccurred())
	})

	It("runner run errs when actions run second return type is not error", func() {
		runner := action.NewRunner()
		_, err := runner.Run(&actionWithSecondReturnValueNotError{}, []byte(`{"arguments":[]}`), 0)
		Expect(err).To(HaveOccurred())
	})

	Describe("Resume", func() {
		It("calls Resume() on action", func() {
			runner := action.NewRunner()
			testAction := &fakeaction.TestAction{
				ResumeErr:   errors.New("fake-action-error"),
				ResumeValue: "fake-action-resume-value",
			}

			value, err := runner.Resume(testAction, []byte{})
			Expect(value).To(Equal("fake-action-resume-value"))
			Expect(err.Error()).To(Equal("fake-action-error"))

			Expect(testAction.Resumed).To(BeTrue())
		})
	})

	It("passes protocol version to run method", func() {
		runner := action.NewRunner()

		actionWithProtocolVersion := &actionWithProtocolVersion{}
		payload := `{"arguments":["setup"]}`

		_, err := runner.Run(actionWithProtocolVersion, []byte(payload), 1)
		Expect(err).ToNot(HaveOccurred())

		Expect(actionWithProtocolVersion.ProtocolVersion).To(Equal(action.ProtocolVersion(1)))
		Expect(actionWithProtocolVersion.SubAction).To(Equal("setup"))
	})

	It("passes protocol version to run method from request ProtocolVersion not the payload", func() {
		runner := action.NewRunner()

		actionWithProtocolVersion := &actionWithProtocolVersion{}
		payload := `{"protocol":98,"arguments":["setup"]}`

		_, err := runner.Run(actionWithProtocolVersion, []byte(payload), 1)
		Expect(err).ToNot(HaveOccurred())

		Expect(actionWithProtocolVersion.ProtocolVersion).To(Equal(action.ProtocolVersion(1)))
		Expect(actionWithProtocolVersion.SubAction).To(Equal("setup"))
	})
})
