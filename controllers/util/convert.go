package util

import (
	"encoding/json"
	"k8s.io/apimachinery/pkg/runtime"
)

var ParameterTag = "parameter"
//
//// Complete do workload definition's rendering
//func Complete(ctx process.Context, name, abstractTemplate string, params interface{}) error {
//	bi := build.NewContext().NewInstance("", nil)
//	if err := bi.AddFile("-", abstractTemplate); err != nil {
//		return errors.WithMessagef(err, "invalid cue template: %s", name)
//	}
//	var paramFile = fmt.Sprintf("%s: {}", ParameterTag)
//	if params != nil {
//		bt, err := json.Marshal(params)
//		if err != nil {
//			return errors.WithMessagef(err, "marshal parameter of configuration %s", name)
//		}
//		if string(bt) != "null" {
//			paramFile = fmt.Sprintf("%s: %s", ParameterTag, string(bt))
//		}
//	}
//	if err := bi.AddFile(ParameterTag, paramFile); err != nil {
//		return errors.WithMessagef(err, "invalid parameter of configuration %s", name)
//	}
//
//	if err := bi.AddFile("-", ctx.BaseContextFile()); err != nil {
//		return err
//	}
//	var r cue.Runtime
//	inst, err := r.Build(bi)
//	if err != nil {
//		return err
//	}
//
//	if err := inst.Value().Err(); err != nil {
//		return errors.WithMessagef(err, "invalid cue template of workload %s after merge parameter and context", wd.name)
//	}
//	output := inst.Lookup(OutputFieldName)
//	base, err := model.NewBase(output)
//	if err != nil {
//		return errors.WithMessagef(err, "invalid output of workload %s", wd.name)
//	}
//	ctx.SetBase(base)
//
//	// we will support outputs for workload composition, and it will become trait in AppConfig.
//	outputs := inst.Lookup(OutputsFieldName)
//	if !outputs.Exists() {
//		return nil
//	}
//	st, err := outputs.Struct()
//	if err != nil {
//		return errors.WithMessagef(err, "invalid outputs of workload %s", wd.name)
//	}
//	for i := 0; i < st.Len(); i++ {
//		fieldInfo := st.Field(i)
//		if fieldInfo.IsDefinition || fieldInfo.IsHidden || fieldInfo.IsOptional {
//			continue
//		}
//		other, err := model.NewOther(fieldInfo.Value)
//		if err != nil {
//			return errors.WithMessagef(err, "invalid outputs(%s) of workload %s", fieldInfo.Name, wd.name)
//		}
//		ctx.AppendAuxiliaries(process.Auxiliary{Ins: other, Type: AuxiliaryWorkload, Name: fieldInfo.Name})
//	}
//	return nil
//}

// RawExtension2Map will convert rawExtension to map
func RawExtension2Map(raw *runtime.RawExtension) (map[string]interface{}, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := raw.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var ret map[string]interface{}
	err = json.Unmarshal(data, &ret)
	if err != nil {
		return nil, err
	}
	return ret, err
}