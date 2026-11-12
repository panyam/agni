v 20200319 2
C 1000 1000 1 0 0 resistor-1.sym
{
T 1100 1400 5 10 1 1 0 0 1
refdes=R1
}
C 1000 2000 1 0 0 resistor-1.sym
{
T 1100 2400 5 10 1 1 0 0 1
refdes=R2
}
C 500 1100 1 0 0 busripper-1.sym
C 500 2100 1 0 0 busripper-1.sym
U 500 800 500 2800 4 0
{
T 550 850 5 10 1 1 0 0 1
netname=DATA[1:0]
}
N 500 1100 1000 1100 4
{
T 600 1150 5 10 1 1 0 0 1
netname=DATA0
}
N 500 2100 1000 2100 4
{
T 600 2150 5 10 1 1 0 0 1
netname=DATA1
}
N 1900 1100 1900 2100 4
{
T 1950 1150 5 10 1 1 0 0 1
netname=OUT
}
