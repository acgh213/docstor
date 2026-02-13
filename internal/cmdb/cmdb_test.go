package cmdb_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/cmdb"
	"github.com/acgh213/docstor/internal/testutil"
)

func TestSystems_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := cmdb.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "CMDBSys", "sys@cmdb.com", "editor")

	// Create
	sys, err := repo.CreateSystem(ctx, cmdb.CreateSystemInput{
		TenantID:   f.TenantID,
		SystemType: "server",
		Name:       "web-01",
		IP:         "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}
	if sys.Name != "web-01" {
		t.Errorf("Name = %q, want web-01", sys.Name)
	}

	// Get
	got, err := repo.GetSystem(ctx, f.TenantID, sys.ID)
	if err != nil {
		t.Fatalf("GetSystem: %v", err)
	}
	if got.IP != "10.0.0.1" {
		t.Errorf("IP = %q, want 10.0.0.1", got.IP)
	}

	// Update
	updated, err := repo.UpdateSystem(ctx, f.TenantID, sys.ID, cmdb.UpdateSystemInput{
		SystemType: "server",
		Name:       "web-01",
		IP:         "10.0.0.2",
	})
	if err != nil {
		t.Fatalf("UpdateSystem: %v", err)
	}
	if updated.IP != "10.0.0.2" {
		t.Errorf("IP after update = %q, want 10.0.0.2", updated.IP)
	}

	// List
	list, err := repo.ListSystems(ctx, f.TenantID, nil)
	if err != nil {
		t.Fatalf("ListSystems: %v", err)
	}
	found := false
	for _, s := range list {
		if s.ID == sys.ID {
			found = true
		}
	}
	if !found {
		t.Error("system not found in list")
	}

	// Delete
	if err := repo.DeleteSystem(ctx, f.TenantID, sys.ID); err != nil {
		t.Fatalf("DeleteSystem: %v", err)
	}
	_, err = repo.GetSystem(ctx, f.TenantID, sys.ID)
	if err != cmdb.ErrNotFound {
		t.Errorf("after delete: got err %v, want ErrNotFound", err)
	}
}

func TestVendors_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := cmdb.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "CMDBVen", "ven@cmdb.com", "editor")

	v, err := repo.CreateVendor(ctx, cmdb.CreateVendorInput{
		TenantID:   f.TenantID,
		Name:       "Acme ISP",
		VendorType: "isp",
		Phone:      "555-1234",
	})
	if err != nil {
		t.Fatalf("CreateVendor: %v", err)
	}

	got, err := repo.GetVendor(ctx, f.TenantID, v.ID)
	if err != nil {
		t.Fatalf("GetVendor: %v", err)
	}
	if got.Phone != "555-1234" {
		t.Errorf("Phone = %q, want 555-1234", got.Phone)
	}

	if err := repo.DeleteVendor(ctx, f.TenantID, v.ID); err != nil {
		t.Fatalf("DeleteVendor: %v", err)
	}
}

func TestContacts_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := cmdb.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "CMDBCon", "con@cmdb.com", "editor")

	c, err := repo.CreateContact(ctx, cmdb.CreateContactInput{
		TenantID: f.TenantID,
		Name:     "Jane Doe",
		Role:     "CTO",
		Email:    "jane@example.com",
	})
	if err != nil {
		t.Fatalf("CreateContact: %v", err)
	}

	got, err := repo.GetContact(ctx, f.TenantID, c.ID)
	if err != nil {
		t.Fatalf("GetContact: %v", err)
	}
	if got.Role != "CTO" {
		t.Errorf("Role = %q, want CTO", got.Role)
	}

	if err := repo.DeleteContact(ctx, f.TenantID, c.ID); err != nil {
		t.Fatalf("DeleteContact: %v", err)
	}
}

func TestCircuits_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := cmdb.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "CMDBCir", "cir@cmdb.com", "editor")

	c, err := repo.CreateCircuit(ctx, cmdb.CreateCircuitInput{
		TenantID:  f.TenantID,
		Provider:  "AT&T",
		CircuitID: "CIR-001",
		WanIP:     "203.0.113.1",
		Speed:     "1Gbps",
	})
	if err != nil {
		t.Fatalf("CreateCircuit: %v", err)
	}

	got, err := repo.GetCircuit(ctx, f.TenantID, c.ID)
	if err != nil {
		t.Fatalf("GetCircuit: %v", err)
	}
	if got.WanIP != "203.0.113.1" {
		t.Errorf("WanIP = %q, want 203.0.113.1", got.WanIP)
	}

	if err := repo.DeleteCircuit(ctx, f.TenantID, c.ID); err != nil {
		t.Fatalf("DeleteCircuit: %v", err)
	}
}

func TestCMDB_TenantIsolation(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := cmdb.NewRepository(pool)
	ctx := context.Background()

	fA := testutil.QuickFixture(t, pool, "CMDBIsoA", "a@cmdb.com", "editor")
	fB := testutil.QuickFixture(t, pool, "CMDBIsoB", "b@cmdb.com", "editor")

	// Tenant A creates a system
	sys, err := repo.CreateSystem(ctx, cmdb.CreateSystemInput{
		TenantID:   fA.TenantID,
		SystemType: "firewall",
		Name:       "fw-01",
	})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}

	// Tenant B cannot get it
	_, err = repo.GetSystem(ctx, fB.TenantID, sys.ID)
	if err != cmdb.ErrNotFound {
		t.Errorf("tenant B GetSystem: got %v, want ErrNotFound", err)
	}

	// Tenant B cannot list it
	list, err := repo.ListSystems(ctx, fB.TenantID, nil)
	if err != nil {
		t.Fatalf("ListSystems B: %v", err)
	}
	for _, s := range list {
		if s.ID == sys.ID {
			t.Error("tenant B can see tenant A's system in list")
		}
	}

	// Tenant B cannot delete it
	err = repo.DeleteSystem(ctx, fB.TenantID, sys.ID)
	if err != cmdb.ErrNotFound {
		t.Errorf("tenant B DeleteSystem: got %v, want ErrNotFound", err)
	}

	// Same for vendors
	v, _ := repo.CreateVendor(ctx, cmdb.CreateVendorInput{
		TenantID: fA.TenantID, Name: "Vendor A", VendorType: "msp",
	})
	_, err = repo.GetVendor(ctx, fB.TenantID, v.ID)
	if err != cmdb.ErrNotFound {
		t.Errorf("tenant B GetVendor: got %v, want ErrNotFound", err)
	}

	// Same for contacts
	con, _ := repo.CreateContact(ctx, cmdb.CreateContactInput{
		TenantID: fA.TenantID, Name: "Contact A", Role: "tech",
	})
	_, err = repo.GetContact(ctx, fB.TenantID, con.ID)
	if err != cmdb.ErrNotFound {
		t.Errorf("tenant B GetContact: got %v, want ErrNotFound", err)
	}

	// Same for circuits
	cir, _ := repo.CreateCircuit(ctx, cmdb.CreateCircuitInput{
		TenantID: fA.TenantID, Provider: "Verizon", CircuitID: "C-1",
	})
	_, err = repo.GetCircuit(ctx, fB.TenantID, cir.ID)
	if err != cmdb.ErrNotFound {
		t.Errorf("tenant B GetCircuit: got %v, want ErrNotFound", err)
	}

	_ = uuid.Nil // keep import
}
