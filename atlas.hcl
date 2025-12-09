// data "external_schema" "gorm" {
//   program = [
//     "go",
//     "run",
//     "-mod=mod",
//     "ariga.io/atlas-provider-gorm",
//     "load",
//     "--path", "./internal/data/model",
//     "--dialect", "postgres",
//   ]
// }
data "external_schema" "gorm" {
  program = [
    "go",
    "run",
    "-mod=mod",
    "./atlas-loader",
  ]
}
env "local" {
  src = data.external_schema.gorm.url
  url = "postgres://inventory_api:postgres@localhost:5435/inventory?search_path=public&sslmode=disable"
  dev = "postgres://inventory_api:postgres@localhost:5435/inventory?search_path=public&sslmode=disable"
  migration {
    dir = "file://migrations"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}