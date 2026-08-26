package main

import "testing"

func TestReusePreviousDeviceConfigClearsOldUploadSession(t *testing.T) {
	service := &Service{
		configs:         map[string]DeviceConfigDTO{"COM3": {DeviceID: "COM3", PortName: "COM3", BaudRate: 115200}},
		previousConfigs: map[string]DeviceConfigDTO{"COM3": {DeviceID: "COM3", PortName: "COM3", Configured: true, ProjectID: "p1", ProjectName: "项目一", Version: "V1", UploaderName: "张三", UploaderEmail: "user@example.com", Remark: "夜间测试", UploadEnabled: true, UploadSessionID: "old-session", QueryCode: "DR123"}},
	}
	value, err := service.ReusePreviousDeviceConfig("COM3")
	if err != nil {
		t.Fatal(err)
	}
	if value.Configured || value.ProjectID != "p1" || value.UploaderEmail != "user@example.com" || value.Remark != "夜间测试" {
		t.Fatalf("business fields were not reused: %#v", value)
	}
	if value.UploadSessionID != "" || value.QueryCode != "" {
		t.Fatalf("old upload session should not be reused: %#v", value)
	}
}
