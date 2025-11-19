package service

// Keeping this around for other Service Related Options

type Options struct {
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags() {
}

func (o *Options) Validate() []error {
	var errs []error
	return errs
}

func (o *Options) Complete() []error {
	return nil
}
