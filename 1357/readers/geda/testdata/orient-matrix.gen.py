# Writes orient-matrix.sch: one asymmetric part at each angle, unmirrored and mirrored, with a
# labelled net stub at every place each pin could land under either order of mirror and rotate.
pins = [(1, 0, 0), (2, 600, 0), (3, 0, 400)]   # number, connect point in the symbol's frame
def rot(x, y, a):
    return {0: (x, y), 90: (-y, x), 180: (-x, -y), 270: (y, -x)}[a]
sym = ['v 20200319 2']
for n, x, y in pins:
    # whichend 0: (x1,y1) is the connect end; the pin runs 200 inward toward the body at (300,200)
    dx = 0 if x == 0 and y == 400 else (200 if x == 0 else -200)
    dy = -200 if (x, y) == (0, 400) else 0
    sym += [f'P {x} {y} {x+dx} {y+dy} 1 0 0', '{', f'T {x+50} {y+50} 5 8 1 1 0 0 1', f'pinnumber={n}', f'T {x+50} {y+50} 5 8 0 1 0 0 1', f'pinseq={n}', f'T {x+50} {y+50} 5 8 0 1 0 0 1', f'pinlabel=P{n}', '}']
sym += ['B 200 -100 400 400 3 0 0 0 -1 -1 0 -1 -1 -1 -1 -1', 'T 200 450 8 10 1 1 0 0 1', 'refdes=U?', 'T 200 550 8 10 0 1 0 0 1', 'device=TRI']
open('tri.sym', 'w').write('\n'.join(sym) + '\n')
sch = ['v 20200319 2']
k = 0
for mirror in (0, 1):
    for a in (0, 90, 180, 270):
        k += 1
        ox, oy = 3000 * (k - 1) % 12000 + 2000, 2000 + 4000 * mirror
        ref = f'U{k}'
        sch += [f'C {ox} {oy} 1 {a} {mirror} tri.sym', '{', f'T {ox} {oy+800} 5 10 1 1 {a} 0 1', f'refdes={ref}', '}']
        cands = set()
        for n, x, y in pins:
            mx = -x if mirror else x
            cands.add(rot(mx, y, a))                      # mirror, then rotate
            rx, ry = rot(x, y, a); cands.add((-rx if mirror else rx, ry))  # rotate, then mirror
        for (cx, cy) in sorted(cands):
            X, Y = ox + cx, oy + cy
            sch += [f'N {X} {Y} {X-100} {Y-100} 4', '{', f'T {X-100} {Y-100} 5 10 0 1 0 0 1', f'netname={ref}_{cx}_{cy}'.replace('-', 'm'), '}']
open('orient-matrix.sch', 'w').write('\n'.join(sch) + '\n')
