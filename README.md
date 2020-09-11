# Coder Camp Compliance

## Introduction

Requested by Chad.

The purpose of this application is to look at specific rule violations in NCM,
extract the details of the most recent rule violations for each node via SWQL,
and then extract the interfaces that are in violation.

## Output

The output should be in CSV format with the headings "Node Name", "Interface
Name".

### Example

```csv
Node Name,Interface Name
RouterA,GigabitEthernet 1/0/3
RouterA,GigabitEthernet 1/0/4
RouterA,GigabitEthernet 1/0/5
RouterB,GigabitEthernet 2/0/4
RouterB,GigabitEthernet 2/0/5
RouterB,GigabitEthernet 2/0/6
```

## Enhancements

Add an option to report on this information historically.

## Possible Other Formats

```csv
Node Name,Rule Name,Code Block,Pattern Name,Pattern,InViolation,LineNumber
RouterA,ISE Imaging Port Detection,interface GigabitEthernet5/0/8,switchport access vlan,True,5184
RouterA,ISE Imaging Port Detection,interface GigabitEthernet5/0/8,authentication event server dead action authorize vlan,False,
RouterA,ISE Imaging Port Detection,interface GigabitEthernet5/0/8,authentication event fail action next-method,False,
RouterA,ISE Imaging Port Detection,interface GigabitEthernet5/0/8,switchport mode trunk,False,
RouterA,ISE Imaging Port Detection,interface GigabitEthernet5/0/8,switchport mode access,True,5185
```

## Features

1. Accept rule name to filter down selection.
2. Default to all rules if no rule is specified.
