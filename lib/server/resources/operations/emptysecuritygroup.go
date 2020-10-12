package operations

import (
	"github.com/CS-SI/SafeScale/lib/protocol"
	"github.com/CS-SI/SafeScale/lib/server/iaas"
	"github.com/CS-SI/SafeScale/lib/server/resources"
	"github.com/CS-SI/SafeScale/lib/server/resources/abstract"
	propertiesv1 "github.com/CS-SI/SafeScale/lib/server/resources/properties/v1"
	"github.com/CS-SI/SafeScale/lib/utils/concurrency"
	"github.com/CS-SI/SafeScale/lib/utils/data"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
)

type emptysecuritygroup struct {
}

func (e emptysecuritygroup) Serialize(task concurrency.Task) ([]byte, fail.Error) {
	return nil, nil
}

func (e emptysecuritygroup) Deserialize(task concurrency.Task, bytes []byte) fail.Error {
	return nil
}

func (e emptysecuritygroup) GetService() iaas.Service {
	panic("implement me")
}

func (e emptysecuritygroup) Inspect(task concurrency.Task, callback resources.Callback) fail.Error {
	return nil
}

func (e emptysecuritygroup) Alter(task concurrency.Task, callback resources.Callback) fail.Error {
	return nil
}

func (e emptysecuritygroup) Carry(task concurrency.Task, clonable data.Clonable) fail.Error {
	return nil
}

func (e emptysecuritygroup) Read(task concurrency.Task, ref string) fail.Error {
	return nil
}

func (e emptysecuritygroup) Reload(task concurrency.Task) fail.Error {
	return nil
}

func (e emptysecuritygroup) BrowseFolder(task concurrency.Task, callback func(buf []byte) fail.Error) fail.Error {
	return nil
}

func (e emptysecuritygroup) Delete(task concurrency.Task) fail.Error {
	return nil
}

func (e emptysecuritygroup) IsNull() bool {
	return false
}

func (e emptysecuritygroup) GetName() string {
	return ""
}

func (e emptysecuritygroup) GetID() string {
	return ""
}

func (e emptysecuritygroup) AddRule(task concurrency.Task, rule abstract.SecurityGroupRule) fail.Error {
	return nil
}

func (e emptysecuritygroup) BindToHost(task concurrency.Task, host resources.Host, disabled bool) fail.Error {
	return nil
}

func (e emptysecuritygroup) BindToNetwork(task concurrency.Task, network resources.Network, disabled bool) fail.Error {
	return nil
}

func (e emptysecuritygroup) Browse(
	task concurrency.Task, callback func(*abstract.SecurityGroup) fail.Error,
) fail.Error {
	return nil
}

func (e emptysecuritygroup) CheckConsistency(task concurrency.Task) fail.Error {
	return nil
}

func (e emptysecuritygroup) Clear(task concurrency.Task) fail.Error {
	return nil
}

func (e emptysecuritygroup) Create(
	task concurrency.Task, name, description string, rules []abstract.SecurityGroupRule,
) fail.Error {
	return nil
}

func (e emptysecuritygroup) DeleteRule(task concurrency.Task, ruleID string) fail.Error {
	return nil
}

func (e emptysecuritygroup) GetBoundHosts(task concurrency.Task) ([]*propertiesv1.SecurityGroupBond, fail.Error) {
	return nil, nil
}

func (e emptysecuritygroup) GetBoundNetworks(task concurrency.Task) ([]*propertiesv1.SecurityGroupBond, fail.Error) {
	return nil, nil
}

func (e emptysecuritygroup) Remove(task concurrency.Task, force bool) fail.Error {
	return nil
}

func (e emptysecuritygroup) Reset(task concurrency.Task) fail.Error {
	return nil
}

func (e emptysecuritygroup) UnbindFromHost(task concurrency.Task, host resources.Host) fail.Error {
	return nil
}

func (e emptysecuritygroup) UnbindFromNetwork(task concurrency.Task, network resources.Network) fail.Error {
	return nil
}

func (e emptysecuritygroup) ToProtocol(task concurrency.Task) (*protocol.SecurityGroupResponse, fail.Error) {
	return nil, nil
}
