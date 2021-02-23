package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/alecthomas/kong"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
)

// const for common action in stored procedure
const (
	SELECT = iota
	INSERT = iota
	UPDATE = iota
	DELETE = iota
)

var (
	cli struct {
		ConnectionString string `arg:"" help:"Connection string"`
		DriverName       string `arg:"" help:"Driver name"`
		TableName        string `arg:"" help:"Table name"`
		Select           bool   `help:"Add select action into stored procedure" short:"s"`
		Update           bool   `help:"Add update action into stored procedure" short:"u"`
		Delete           bool   `help:"Add delete action into stored procedure" short:"d"`
		Insert           bool   `help:"Add insert action into stored procedure" short:"i"`
		ProcedureName    string `help:"Stored Procedure Name" short:"p"`
		OutputFile       string `help:"Output File" short:"o"`
	}
)

// Column is the struct with description from DataBase
type Column struct {
	ColumnName    string
	ColumnType    string
	IsIdentifier  bool
	ParameterName string
}

// Table is the struct with description from DataBase
type Table struct {
	Columns   []Column
	TableName string
}

// TemplateSettings is structure to define where template files are located
type TemplateSettings struct {
	Folder         string
	SelectTemplate string
	InsertTemplate string
	UpdateTemplate string
	DeleteTemplate string
}

// StoreProcedureStructure is structure that represent the store procedure
type StoreProcedureStructure struct {
	Action           int
	ParameterSection string
	Query            string
	WhereCondition   string
}

// GenerateOption is structure that control how the store procedure will be generated
type GenerateOption struct {
	Actions            []int
	SQLServer          string
	StoreProcedureName string
	ActionParameter    string
	Template           TemplateSettings
}

func (settings *TemplateSettings) getTemplateFileFor(action int) string {
	var template string
	switch action {
	case SELECT:
		template = settings.SelectTemplate
	case INSERT:
		template = settings.InsertTemplate
	case DELETE:
		template = settings.DeleteTemplate
	case UPDATE:
		template = settings.UpdateTemplate
	}
	return filepath.Join(settings.Folder, template)
}

func getStoreProcedureSuffix(action int) string {
	switch action {
	case SELECT:
		return "_select"
	case INSERT:
		return "_insert"
	case DELETE:
		return "_delete"
	case UPDATE:
		return "_update"
	}
	return ""
}

func (settings *TemplateSettings) getParameterSectionTemplateFile(driverName string) string {
	return filepath.Join(settings.Folder, driverName, "parameterSection.tmpl")
}

func (settings *TemplateSettings) getParameterTemplateFile(driverName string) string {
	return filepath.Join(settings.Folder, driverName, "parameterConvert.tmpl")
}

func (settings *TemplateSettings) getStoredProcedureTemplateFile(driverName string) string {
	return filepath.Join(settings.Folder, driverName, "procedure.tmpl")
}

func generateParameterName(templateFile string, columnName string) (string, error) {
	return generateStringFromFileTemplate(templateFile, columnName)
}

func generateStringFromTemplate(tmpl string, model interface{}) string {
	t := template.Must(template.New("tmpl").Funcs(funcMap).Parse(string(tmpl)))
	buf := new(bytes.Buffer)
	t.Execute(buf, model)
	return buf.String()
}

type parameterNameProvider func(string) string

func generateStringFromFileTemplate(templateFile string, model interface{}) (string, error) {
	tmpl, err := ioutil.ReadFile(templateFile)

	exitIfError(err)

	return generateStringFromTemplate(string(tmpl), model), nil
}

func (option *GenerateOption) generateWhereCondition(table Table) string {

	idColumns := make([]Column, 0)
	for _, col := range table.Columns {
		if col.IsIdentifier {
			idColumns = append(idColumns, col)
		}
	}
	if len(idColumns) == 0 {
		return ""
	}
	return generateStringFromTemplate(`{{$cnt:=(minus (len .) 1)}}WHERE ({{range $index, $val := .}}({{$val.ColumnName}} = {{$val.ParameterName}} OR {{$val.ParameterName}} IS NULL){{if (ne $index $cnt)}} AND {{end}}{{end}})`, idColumns)

}

// GenerateFile is function for creating resulting file content
func (option *GenerateOption) GenerateFile(structure StoreProcedureStructure) string {

	result, _ := generateStringFromFileTemplate(option.Template.getStoredProcedureTemplateFile(option.SQLServer), model{StoreProcedureName: *&option.StoreProcedureName + getStoreProcedureSuffix(structure.Action), Structure: structure})
	return result
}

type filterfn func(Column) bool

func filterColumns(columns []Column, filter filterfn) []Column {
	temp := make([]Column, 0)
	for _, column := range columns {
		if filter(column) {
			temp = append(temp, column)
		}
	}

	return temp
}

func filterColumnByAction(action int, columns []Column) []Column {
	if action == UPDATE || action == INSERT {
		return filterColumns(columns, func(column Column) bool {
			return !column.IsIdentifier
		})
	} else if action == DELETE {
		return filterColumns(columns, func(column Column) bool {
			return column.IsIdentifier
		})
	}
	return columns
}

// GenerateSQL is function for creating stored procedures
func (settings *TemplateSettings) GenerateSQL(table Table, option GenerateOption) []StoreProcedureStructure {

	procedures := make([]StoreProcedureStructure, len(option.Actions))

	for i, action := range option.Actions {

		tableModel := Table{TableName: table.TableName, Columns: filterColumnByAction(action, table.Columns)}

		query, err := generateStringFromFileTemplate(settings.getTemplateFileFor(action), tableModel)
		exitIfError(err)

		if action == UPDATE {
			tableModel = table
		}

		whereCondition := option.generateWhereCondition(tableModel)
		parameterSection, _ := generateStringFromFileTemplate(settings.getParameterSectionTemplateFile(option.SQLServer), tableModel)

		sp := CreateStoreProcedureForAction(action, query, whereCondition)
		sp.ParameterSection = parameterSection
		sp.Action = action

		procedures[i] = sp
	}

	return procedures
}

type model struct {
	StoreProcedureName string
	Structure          StoreProcedureStructure
}

// CreateStoreProcedureForAction is function for creating structure if store procedure for requested action (select, insert, update)
func CreateStoreProcedureForAction(action int, query string, whereCondition string) StoreProcedureStructure {
	if action == INSERT {
		return StoreProcedureStructure{Query: query}
	}

	return StoreProcedureStructure{Query: query, WhereCondition: whereCondition}
}

func exitIfError(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

// GetTableDescription is the method
func GetTableDescription(db *sql.DB, table string, provider parameterNameProvider) Table {
	result := make([]Column, 0)
	sqlquery := `SELECT col.COLUMN_NAME AS ColumnName, col.DATA_TYPE AS DataType, col.CHARACTER_MAXIMUM_LENGTH AS MaxLength, CASE WHEN con.CONSTRAINT_NAME IS NULL THEN 0 ELSE 1 END AS IsPrimaryKey
	FROM INFORMATION_SCHEMA.COLUMNS  col
	 LEFT JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ke ON col.COLUMN_NAME = ke.COLUMN_NAME
	 														AND col.TABLE_SCHEMA = ke.CONSTRAINT_SCHEMA 
															AND col.TABLE_NAME = ke.TABLE_NAME
	 LEFT JOIN INFORMATION_SCHEMA.TABLE_CONSTRAINTS  con ON con.TABLE_NAME = ke.TABLE_NAME
															AND con.CONSTRAINT_CATALOG = ke.CONSTRAINT_CATALOG
															AND con.CONSTRAINT_SCHEMA = ke.CONSTRAINT_SCHEMA
															AND con.CONSTRAINT_NAME = ke.CONSTRAINT_NAME
															AND con.CONSTRAINT_TYPE = 'PRIMARY KEY'
	WHERE col.TABLE_NAME=?`
	rows, err := db.Query(sqlquery, table)

	exitIfError(err)
	defer rows.Close()
	var (
		name      string
		typeName  string
		maxLength sql.NullInt64
		isPrimary bool
	)
	for rows.Next() {
		err := rows.Scan(&name, &typeName, &maxLength, &isPrimary)
		exitIfError(err)

		if maxLength.Valid {
			if maxLength.Int64 == -1 {
				typeName = typeName + "(max)"
			} else {
				typeName = typeName + "(" + strconv.FormatInt(maxLength.Int64, 10) + ")"
			}
		}

		col := Column{ColumnName: name, ColumnType: typeName, IsIdentifier: isPrimary, ParameterName: provider(name)}
		result = append(result, col)
	}
	err = rows.Err()
	exitIfError(err)
	return Table{Columns: result, TableName: table}
}

var funcMap = template.FuncMap{
	"minus": minus,
}

func minus(a, b int) int {
	return a - b
}

func main() {

	settings := TemplateSettings{
		Folder:         "templates",
		SelectTemplate: "select.tmpl",
		InsertTemplate: "insert.tmpl",
		UpdateTemplate: "update.tmpl",
		DeleteTemplate: "delete.tmpl",
	}

	kong.Parse(&cli)

	actions := make([]int, 0)

	if cli.Select {
		actions = append(actions, SELECT)
	}
	if cli.Delete {
		actions = append(actions, DELETE)
	}
	if cli.Insert {
		actions = append(actions, INSERT)
	}
	if cli.Update {
		actions = append(actions, UPDATE)
	}

	if cli.TableName == "" {
		fmt.Print("Table is not specified")
	}

	if cli.ConnectionString == "" {
		fmt.Print("connection is not specified")
	}

	procedureName := cli.ProcedureName

	if procedureName == "" {
		procedureName = "sp_" + cli.TableName
	}

	actionParameter, _ := generateParameterName(settings.getParameterTemplateFile(cli.DriverName), "action")

	option := GenerateOption{
		SQLServer:          cli.DriverName,
		StoreProcedureName: procedureName,
		Actions:            actions,
		Template:           settings,
		ActionParameter:    actionParameter,
	}

	db, err := sql.Open(option.SQLServer, cli.ConnectionString)

	exitIfError(err)
	defer db.Close()

	sp := settings.GenerateSQL(GetTableDescription(db, cli.TableName, func(name string) string {
		paramName, _ := generateParameterName(settings.getParameterTemplateFile(option.SQLServer), name)
		return paramName
	}), option)

	for _, file := range sp {

		result := option.GenerateFile(file)

		if cli.OutputFile != "" {
			err = ioutil.WriteFile(cli.OutputFile+getStoreProcedureSuffix(file.Action)+".sql", []byte(result), 0755)
			exitIfError(err)
		} else {
			fmt.Printf("%v\n", result)
		}

	}
}
