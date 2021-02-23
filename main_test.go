package main

import (
	"testing"
)

func createTest(columns []Column, expectedResult int) func(*testing.T) {
	return func(t *testing.T) {
		result := filterColumnByAction(INSERT, columns)
		if len(result) != expectedResult {
			t.Fatalf("The removing of column has unexpected result:%v", result)
		}
	}
}

func TestRemovePrimaryKeyColumns(t *testing.T) {
	t.Run("single non PK column", createTest([]Column{{ColumnName: "test", ColumnType: "int", IsIdentifier: false, ParameterName: ""}}, 1))
	t.Run("single PK column", createTest([]Column{{ColumnName: "test", ColumnType: "int", IsIdentifier: true, ParameterName: ""}}, 0))
	t.Run("last PK column", createTest([]Column{{ColumnName: "test", ColumnType: "int", IsIdentifier: false, ParameterName: ""}, {ColumnName: "test", ColumnType: "int", IsIdentifier: true, ParameterName: ""}}, 1))
}

func TestAddWhereConditionToSelect(t *testing.T) {
	where := "where id = @id"
	sql := "select * from"

	query := CreateStoreProcedureForAction(SELECT, sql, where)

	if query.Query != sql {
		t.Fatal("Query is not added")
	}
	if query.WhereCondition != where {
		t.Fatal("Where condition is not added to query")
	}
}

func TestAddWhereConditionToUpdate(t *testing.T) {
	where := "where id = @id"
	sql := "select * from"
	query := CreateStoreProcedureForAction(UPDATE, sql, where)

	if query.Query != sql {
		t.Fatal("Query is not added")
	}
	if query.WhereCondition != where {
		t.Fatal("Where condition is not added to query")
	}
}

func TestAddWhereConditionToInsert(t *testing.T) {
	where := "where id = @id"
	sql := "select * from"
	query := CreateStoreProcedureForAction(INSERT, sql, where)

	if query.Query != sql {
		t.Fatal("Query is not added")
	}
	if query.WhereCondition == where {
		t.Fatal("Where condition is added INSERT")
	}
}

func TestAddWhereConditionToDelete(t *testing.T) {
	where := "where id = @id"
	sql := "select * from"
	query := CreateStoreProcedureForAction(DELETE, sql, where)

	if query.Query != sql {
		t.Fatal("Query is not added")
	}
	if query.WhereCondition != where {
		t.Fatal("Where condition is not added to query")
	}
}
