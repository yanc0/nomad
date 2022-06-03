package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestSecureVariables_SimpleCRUD(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	nsv := c.SecureVariables()
	sv1 := &SecureVariable{
		Path: "my/variable/a",
		Items: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	t.Run("1 fail create when no items", func(t *testing.T) {
		_, err := nsv.Create(&SecureVariable{Path: "bad/var"}, nil)
		require.Error(t, err)
	})
	t.Run("2 create sv1", func(t *testing.T) {
		_, err := nsv.Create(sv1, nil)
		require.NoError(t, err)
		get, _, err := nsv.Read(sv1.Path, nil)
		require.NoError(t, err)
		require.NotNil(t, get)
		require.Equal(t, sv1.Items, get.Items)
		sv1 = get
	})
	t.Run("3 update sv1 no change", func(t *testing.T) {
		_, err := nsv.Update(sv1, nil)
		require.NoError(t, err)
		get, _, err := nsv.Read(sv1.Path, nil)
		require.NotNil(t, get)
		require.Equal(t, sv1.ModifyIndex, get.ModifyIndex, "ModifyIndex should not change")
		require.Equal(t, sv1.Items, get.Items)
		sv1 = get
	})
	t.Run("4 update sv1", func(t *testing.T) {
		sv1.Items["new-hotness"] = "yeah!"
		_, err := nsv.Update(sv1, nil)
		require.NoError(t, err)
		get, _, err := nsv.Read(sv1.Path, nil)
		require.NotNil(t, get)
		require.NotEqual(t, sv1.ModifyIndex, get.ModifyIndex, "ModifyIndex should change")
		require.Equal(t, sv1.Items, get.Items)
		sv1 = get
	})
	t.Run("5 list vars", func(t *testing.T) {
		l, _, err := nsv.List(nil)
		require.NoError(t, err)
		require.Len(t, l, 1)
		require.Equal(t, sv1.Stub(), l[0])
	})
	t.Run("6 delete sv1", func(t *testing.T) {
		_, err := nsv.PurgeVariableAtPath(sv1.Path, nil)
		require.NoError(t, err)
		get, _, err := nsv.Read(sv1.Path, nil)
		require.Error(t, err)
		require.Nil(t, get)
	})
	t.Run("7 list vars after delete", func(t *testing.T) {
		l, _, err := nsv.List(nil)
		require.NoError(t, err)
		require.NotNil(t, l)
		require.Len(t, l, 0)
	})
}

func TestSecureVariables_CRUDWithCAS(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	nsv := c.SecureVariables()
	sv1 := &SecureVariable{
		Path: "cas/variable/a",
		Items: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	t.Run("1 create sv1", func(t *testing.T) {
		_, err := nsv.Create(sv1, nil)
		require.NoError(t, err)
		get, _, err := nsv.Read(sv1.Path, nil)
		require.NoError(t, err)
		require.NotNil(t, get)
		require.Equal(t, sv1.Items, get.Items)
	})

	t.Run("2 create sv1 again", func(t *testing.T) {
		_, err := nsv.Create(sv1, nil)
		require.Error(t, err)
	})

	t.Run("3 update sv1 with CAS", func(t *testing.T) {
		oobUpdate := sv1.Copy()
		// perform out of band upsert
		oobUpdate.Items["new-hotness"] = "yeah!"
		_, err := nsv.Upsert(oobUpdate, nil)
		require.NoError(t, err)

		// try to do an update
		_, err = nsv.Update(sv1, nil)
		require.Error(t, err)
	})

	t.Run("4 delete old index", func(t *testing.T) {
		_, err := nsv.PurgeWithCheckIndex(sv1.Path, sv1.ModifyIndex, nil)
		require.Error(t, err)
	})
	t.Run("5 delete current index", func(t *testing.T) {
		get, _, err := nsv.Read(sv1.Path, nil)
		require.NoError(t, err)
		_, err = nsv.PurgeWithCheckIndex(get.Path, get.ModifyIndex, nil)
		require.NoError(t, err)
	})
}
