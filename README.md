# GoONT - Herramienta de Gestión de OLT/ONT

GoONT es una herramienta de línea de comandos (CLI) escrita en Go para la gestión y monitoreo de equipos OLT (Optical Line Terminal) y ONT (Optical Network Terminal) en redes GPON mediante protocolo SNMP.

## Características

- **Gestión de OLTs**: Agregar, listar y eliminar OLTs de la base de datos
- **Escaneo de ONTs**: Obtención automática de información de ONTs conectados a cada OLT
- **Almacenamiento local**: SQLite para persistencia de datos
- **Consultas SNMP**: Soporte para SNMP v2c con timeouts y reintentos configurables
- **Concurrente**: Escaneo paralelo de múltiples OLTs

## Instalación

### Requisitos previos
- Go 1.25.6 o superior

### Desde código fuente
```bash
git clone https://github.com/metalpoch/goont
cd goont
go build -o goont ./cmd/cli
```

## Uso

### Comandos disponibles

```bash
# Gestionar OLTs
goont olt list          # Listar todos los OLTs registrados
goont olt add           # Agregar un nuevo OLT
goont olt remove        # Eliminar un OLT

# Escanear ONTs
goont ont scan          # Escanear ONTs en todos los OLTs registrados
```

### Agregar un OLT
```bash
goont olt add --ip 192.168.1.1 --community public --timeout 60 --retries 3
```

### Escanear ONTs
```bash
goont ont scan
```

## Estructura del Proyecto

```
goont/
├── cmd/cli/main.go    # Punto de entrada principal
├── commands/          # Implementación de comandos CLI
│   ├── olt.go         # Comandos para OLT
│   ├── ont.go         # Comandos para ONT
│   └── utils.go       # Funciones auxiliares
├── snmp/              # Lógica de consultas SNMP
│   ├── snmp.go        # Cliente SNMP
│   └── types.go       # Estructuras de datos SNMP
├── storage/           # Capa de almacenamiento
│   ├── olt.go         # Operaciones CRUD para OLTs
│   ├── ont.go         # Operaciones CRUD para ONTs
│   ├── types.go       # Estructuras de datos
│   └── utils.go       # Utilidades de base de datos
└── go.mod             # Definición del módulo Go
```

## Base de Datos

La herramienta utiliza SQLite para almacenamiento local:

- `olt.db`: Base de datos principal con información de OLTs
- Archivos separados por IP para mediciones de ONTs

## Dependencias

- [gosnmp/gosnmp](https://github.com/gosnmp/gosnmp): Cliente SNMP
- [urfave/cli](https://github.com/urfave/cli): Framework CLI
- [modernc.org/sqlite](https://modernc.org/sqlite): Driver SQLite puro en Go
- [olekukonko/tablewriter](https://github.com/olekukonko/tablewriter): Tablas en terminal

## Configuración

La configuración se maneja mediante:
- Parámetros de línea de comandos
- Base de datos SQLite automática

## Contribuciones

Las contribuciones son bienvenidas. Por favor, abre un issue o pull request en GitHub.

## Licencia

MIT
