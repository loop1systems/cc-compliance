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

Node Name,Interface Name
RouterA,GigabitEthernet 1/0/3
RouterA,GigabitEthernet 1/0/4
RouterA,GigabitEthernet 1/0/5
RouterB,GigabitEthernet 2/0/4
RouterB,GigabitEthernet 2/0/5
RouterB,GigabitEthernet 2/0/6

## Enhancements

Add an option to report on this information historically.
