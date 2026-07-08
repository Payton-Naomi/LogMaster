package serialagent

import "testing"

func TestListPorts(t *testing.T) {
	ports, err := ListPorts()
	if err != nil {
		t.Fatalf("ListPorts() 返回错误: %v", err)
	}
	if ports == nil {
		t.Fatal("ListPorts() 返回了 nil 切片")
	}
	// 如果没有串口设备，ports 可能为空，这是正常的
	t.Logf("发现 %d 个端口: %v", len(ports), ports)
}