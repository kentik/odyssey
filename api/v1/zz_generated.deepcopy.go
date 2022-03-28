//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2022 KentikLabs

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Fetch) DeepCopyInto(out *Fetch) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Fetch.
func (in *Fetch) DeepCopy() *Fetch {
	if in == nil {
		return nil
	}
	out := new(Fetch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfluxDB) DeepCopyInto(out *InfluxDB) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfluxDB.
func (in *InfluxDB) DeepCopy() *InfluxDB {
	if in == nil {
		return nil
	}
	out := new(InfluxDB)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Ping) DeepCopyInto(out *Ping) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Ping.
func (in *Ping) DeepCopy() *Ping {
	if in == nil {
		return nil
	}
	out := new(Ping)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyntheticTask) DeepCopyInto(out *SyntheticTask) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyntheticTask.
func (in *SyntheticTask) DeepCopy() *SyntheticTask {
	if in == nil {
		return nil
	}
	out := new(SyntheticTask)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SyntheticTask) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyntheticTaskList) DeepCopyInto(out *SyntheticTaskList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SyntheticTask, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyntheticTaskList.
func (in *SyntheticTaskList) DeepCopy() *SyntheticTaskList {
	if in == nil {
		return nil
	}
	out := new(SyntheticTaskList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SyntheticTaskList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyntheticTaskSpec) DeepCopyInto(out *SyntheticTaskSpec) {
	*out = *in
	if in.ServerCommand != nil {
		in, out := &in.ServerCommand, &out.ServerCommand
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AgentCommand != nil {
		in, out := &in.AgentCommand, &out.AgentCommand
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.InfluxDB != nil {
		in, out := &in.InfluxDB, &out.InfluxDB
		*out = new(InfluxDB)
		**out = **in
	}
	if in.Fetch != nil {
		in, out := &in.Fetch, &out.Fetch
		*out = make([]*Fetch, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(Fetch)
				**out = **in
			}
		}
	}
	if in.TLSHandshake != nil {
		in, out := &in.TLSHandshake, &out.TLSHandshake
		*out = make([]*TLSHandshake, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(TLSHandshake)
				**out = **in
			}
		}
	}
	if in.Trace != nil {
		in, out := &in.Trace, &out.Trace
		*out = make([]*Trace, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(Trace)
				**out = **in
			}
		}
	}
	if in.Ping != nil {
		in, out := &in.Ping, &out.Ping
		*out = make([]*Ping, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(Ping)
				**out = **in
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyntheticTaskSpec.
func (in *SyntheticTaskSpec) DeepCopy() *SyntheticTaskSpec {
	if in == nil {
		return nil
	}
	out := new(SyntheticTaskSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyntheticTaskStatus) DeepCopyInto(out *SyntheticTaskStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyntheticTaskStatus.
func (in *SyntheticTaskStatus) DeepCopy() *SyntheticTaskStatus {
	if in == nil {
		return nil
	}
	out := new(SyntheticTaskStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TLSHandshake) DeepCopyInto(out *TLSHandshake) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TLSHandshake.
func (in *TLSHandshake) DeepCopy() *TLSHandshake {
	if in == nil {
		return nil
	}
	out := new(TLSHandshake)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Trace) DeepCopyInto(out *Trace) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Trace.
func (in *Trace) DeepCopy() *Trace {
	if in == nil {
		return nil
	}
	out := new(Trace)
	in.DeepCopyInto(out)
	return out
}
