v {xschem version=3.4.4 file_version=1.2}
G {}
K {}
V {}
S {}
E {}
N 100 -60 100 -30 {lab=IN}
N 100 30 100 60 {lab=MID}
N 100 120 100 150 {lab=OUT}
C {res.sym} 100 0 0 0 {name=R1
value=1k
device=resistor}
C {res.sym} 100 90 0 0 {name=R2 value=2k device=resistor}
C {ipin.sym} 100 -60 0 0 {name=p1 lab=IN}
C {opin.sym} 100 150 0 0 {name=p2 lab=OUT}
C {gnd.sym} 100 200 0 0 {name=l1 lab=0}
