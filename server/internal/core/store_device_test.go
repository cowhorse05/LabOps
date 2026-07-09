package core

import (
	"context"
	"testing"
)

func TestDeviceCRUD(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(DriverSQLite, ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("init store: %v", err)
	}

	t.Run("TestCreateDevice", func(t *testing.T) {
		d := Device{
			ID: "dev_crud_test", Name: "crud-test-device", GroupName: "crud-lab",
			Hostname: "crud-host", OS: "Ubuntu 24.04", IP: "10.0.99.1",
			CPUCores: 4, MemoryMB: 4096, DiskTotalGB: 128, Status: StatusOffline,
		}
		if err := store.CreateDevice(ctx, d); err != nil {
			t.Fatalf("CreateDevice: %v", err)
		}
		got, ok, _ := store.GetDevice(ctx, d.ID)
		if !ok || got.Name != d.Name || got.Status != StatusOffline {
			t.Fatalf("unexpected device after create: %+v, ok=%v", got, ok)
		}
	})

	t.Run("TestCreateDevice_DuplicateFails", func(t *testing.T) {
		d := Device{ID: "dup_test", Name: "dup-test", GroupName: "lab", Status: StatusOffline}
		store.CreateDevice(ctx, d)
		if err := store.CreateDevice(ctx, d); err == nil {
			t.Fatal("expected duplicate key error")
		}
	})

	t.Run("TestDeleteDevice", func(t *testing.T) {
		d := Device{ID: "del_test", Name: "del-test", GroupName: "lab", Status: StatusOffline}
		store.CreateDevice(ctx, d)
		if err := store.DeleteDevice(ctx, "del_test"); err != nil {
			t.Fatalf("DeleteDevice: %v", err)
		}
		if _, ok, _ := store.GetDevice(ctx, "del_test"); ok {
			t.Fatal("device should not exist after delete")
		}
	})
}
