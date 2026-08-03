v 20200319 2
C 0 0 1 0 0 slotgate.sym
{
T 200 600 5 10 1 1 0 0 1
refdes=U1
T 200 610 5 10 1 0 0 0 1
slot=1
}
C 2000 0 1 0 0 slotgate.sym
{
T 2200 600 5 10 1 1 0 0 1
refdes=U1
T 2200 610 5 10 1 0 0 0 1
slot=2
}
N 0 200 0 500 4
{
T 0 500 5 10 1 1 0 0 1
netname=IN1
}
N 2000 200 2000 500 4
{
T 2000 500 5 10 1 1 0 0 1
netname=IN2
}
N 2700 200 2700 500 4
{
T 2700 500 5 10 1 1 0 0 1
netname=OUT2
}
