package middleware

//func TestValidation_ValidRequest(t *testing.T) {
//	t.Parallel()
//
//	validator, err := protovalidate.New()
//	assert.NoError(t, err)
//
//	m := Validation(validator)
//
//	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
//		return "success", nil
//	}
//
//	resp, err := m(handler)(context.Background(), &resources.CreateRhelHostRequest{
//		RhelHost: &resources.RhelHost{
//			Metadata: &resources.Metadata{},
//			ReporterData: &resources.ReporterData{
//				ReporterType:    1,
//				LocalResourceId: "1",
//			},
//		},
//	})
//	assert.NoError(t, err)
//	assert.Equal(t, "success", resp)
//}
//
//func TestValidation_InvalidRequest(t *testing.T) {
//	t.Parallel()
//
//	validator, err := protovalidate.New()
//	assert.NoError(t, err)
//
//	m := Validation(validator)
//
//	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
//		return nil, nil
//	}
//	resp, err := m(handler)(context.Background(), &resources.CreateRhelHostRequest{})
//	assert.Error(t, err)
//	assert.Equal(t, resp, nil)
//}
