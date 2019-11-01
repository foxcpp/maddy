package exterrors

type fieldsErr interface {
	Fields() map[string]interface{}
}

type unwrapper interface {
	Unwrap() error
}

type fieldsWrap struct {
	err    error
	fields map[string]interface{}
}

func (fw fieldsWrap) Error() string {
	return fw.err.Error()
}

func (fw fieldsWrap) Unwrap() error {
	return fw.err
}

func (fw fieldsWrap) Fields() map[string]interface{} {
	return fw.fields
}

func Fields(err error) map[string]interface{} {
	fields := make(map[string]interface{}, 5)

	for err != nil {
		errFields, ok := err.(fieldsErr)
		if ok {
			for k, v := range errFields.Fields() {
				fields[k] = v
			}
		}

		unwrap, ok := err.(unwrapper)
		if !ok {
			break
		}
		err = unwrap.Unwrap()
	}

	return fields
}

func WithFields(err error, fields map[string]interface{}) error {
	return fieldsWrap{err: err, fields: fields}
}
