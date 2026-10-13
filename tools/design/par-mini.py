"""A static PAR frame drawn with the geometry of www/par/par-view.js (frameSvg):
square-root altitude scale, transition layer, 7 and 15 degree cones, 3 degree
glide path with 0.7 degree margins, azimuth band. For the design system's
ParFrame card and the final screens; the interface draws the real one."""
import math
W=640; LEFT=56; RIGHT=10; EH=150; AZH=34; top=16
R=20.0; L=1.46; ground=392.0; ceil=15000.0; TA=5000; qnh=1018
FTNM=6076.12
plotW=W-LEFT-RIGHT
azTop=top+EH+20; H=azTop+AZH+20
x=lambda u: LEFT+(u+R)/(L+2*R)*plotW
def y(alt):
    h=max(0,min(1,(alt-ground)/(ceil-ground))); return top+EH*(1-math.sqrt(h))
flAlt=lambda fl: fl*100+(qnh-1013.25)*27.3
azHalf=R*math.tan(math.radians(20)); azMid=azTop+AZH/2
yAz=lambda c: azMid-max(-1,min(1,-c/azHalf))*(AZH/2)
o=[]
L_=lambda x1,y1,x2,y2,s,e='': o.append(f'<line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2:.1f}" y2="{y2:.1f}" stroke="{s}" {e}/>')
T=lambda tx,ty,s,f,e='': o.append(f'<text x="{tx:.1f}" y="{ty:.1f}" fill="{f}" {e}>{s}</text>')
P=lambda pts,s,e='': o.append('<polyline fill="none" stroke="%s" points="%s" %s/>'%(s,' '.join(f'{a:.1f},{b:.1f}' for a,b in pts),e))
grid='#262626'; label='#8a8a8a'; amber='#FFC107'; cone='#4a4a4a'
tl=60
# transition layer, level grid
o.append(f'<rect x="{LEFT}" y="{y(flAlt(tl)):.1f}" width="{plotW}" height="{y(TA)-y(flAlt(tl)):.1f}" fill="{amber}" fill-opacity="0.08"/>')
last=[1e9]
def lab(alt,txt,fill,extra=''):
    if last[0]-y(alt)>=12: T(LEFT-6,y(alt)+3.5,txt,fill,'text-anchor="end" '+extra); last[0]=y(alt)
for ft in (1000,2000,3000,4000):
    L_(LEFT,y(ft),LEFT+plotW,y(ft),grid); lab(ft,f'{ft}',label)
L_(LEFT,y(TA),LEFT+plotW,y(TA),amber,'stroke-opacity="0.5" stroke-dasharray="4 3"'); lab(TA,f'TA {TA}',amber,'fill-opacity="0.8"')
for fl in (60,70,80,100,120,150):
    first=fl==tl
    L_(LEFT,y(flAlt(fl)),LEFT+plotW,y(flAlt(fl)),amber if first else grid,'stroke-opacity="0.5"' if first else '')
    lab(flAlt(fl),f'FL{fl:03d}',amber if first else label)
# cones: 7 deg inbound from 26L's threshold, 15 deg outbound from 08R's end
P([(x(-d),y(min(ceil,ground+d*FTNM*math.tan(math.radians(7))))) for d in [i*0.5 for i in range(41)]],cone,'stroke-dasharray="2 4"')
P([(x(L+d),y(min(ceil,ground+d*FTNM*math.tan(math.radians(15))))) for d in [i*0.25 for i in range(81)]],cone,'stroke-dasharray="1 6"')
# glide paths: 26L strong (in use), 08R faint
def glide(sign, u0, strong):
    base=ground+50; path=[];lo=[];hi=[]
    for i in range(81):
        d=i*0.25; uu=u0+sign*d
        if uu<-R or uu>L+R: break
        run=d*FTNM
        path.append((x(uu),y(base+run*math.tan(math.radians(3)))))
        lo.append((x(uu),y(base+run*math.tan(math.radians(2.3)))))
        hi.append((x(uu),y(base+run*math.tan(math.radians(3.7)))))
    col='#4CAF50' if strong else '#555'
    P(lo,col,'stroke-opacity="0.25"'); P(hi,col,'stroke-opacity="0.25"')
    P(path,col,f'stroke-width="{1.5 if strong else 1}" stroke-dasharray="6 4"')
    a=path[16]; T(a[0],a[1]-5,'3°',col,'text-anchor="middle"'+('' if strong else ' fill-opacity="0.7"'))
glide(1,L,False); glide(-1,0,True)
# ground, runway, thresholds, distances
gy=y(ground)
L_(LEFT,gy,LEFT+plotW,gy,'#3a3a3a')
o.append(f'<rect x="{x(0):.1f}" y="{gy-3:.1f}" width="{x(L)-x(0):.1f}" height="5" fill="#bdbdbd" rx="1"/>')
for u in (0,L): L_(x(u),gy-7,x(u),gy+4,'#e0e0e0','stroke-width="1.5"')
T(x(0),gy+15,'26L','#e0e0e0','text-anchor="end" font-weight="bold"'); T(x(L),gy+15,'08R','#e0e0e0','text-anchor="start" font-weight="bold"')
for d in (5,10,15,20):
    for uu in (-d,L+d):
        L_(x(uu),gy,x(uu),gy+4,label); T(x(uu),gy+15,str(d),label,'text-anchor="middle"')
# azimuth band
o.append(f'<rect x="{LEFT}" y="{azTop}" width="{plotW}" height="{AZH}" fill="#111" stroke="{grid}"/>')
L_(LEFT,azMid,LEFT+plotW,azMid,grid)
o.append(f'<rect x="{x(0):.1f}" y="{azMid-2:.1f}" width="{x(L)-x(0):.1f}" height="4" fill="#bdbdbd" rx="1"/>')
for side in (-azHalf,azHalf):
    L_(x(0),azMid,x(-R),yAz(side),cone,'stroke-dasharray="2 4"'); L_(x(L),azMid,x(L+R),yAz(side),cone,'stroke-dasharray="2 4"')
T(LEFT-6,azMid+3.5,'AZ',label,'text-anchor="end"')
# the followed aircraft's track (white, with dots), then the others' short trails
hist=[(19.5,4000,-4.5),(17,4000,-3.6),(15,4000,-2.4),(13,4000,-1.1),(11,3900,-0.3),(9.5,3460,0),(8,2990,0),(7,2680,0),(6.2,2416,0)]
for pts in ([(x(-d),y(a)) for d,a,c in hist],[(x(-d),yAz(c)) for d,a,c in hist]):
    P(pts,'#ffffff','stroke-opacity="0.75" stroke-width="1.5"')
    for a,b in pts: o.append(f'<circle cx="{a:.1f}" cy="{b:.1f}" r="1.4" fill="#ffffff" fill-opacity="0.75"/>')
arr='#4c9ce8'
ac=[('AFR47KX',6.2,2416,0,'2400 ft ↓',True),('EZY82QT',14.0,5000,0.4,'5000 ft ↓',False),('DLH9KR',16.5,flAlt(85),-1.6,'FL085 ↓',False)]
for cs,d,a,c,lvl,sel in ac:
    if not sel: P([(x(-d-0.9),y(a+260)),(x(-d),y(a))],arr,'stroke-opacity="0.35"')
for cs,d,a,c,lvl,sel in ac:
    px,py,ay=x(-d),y(a),yAz(c)
    if sel: o.append(f'<circle cx="{px:.1f}" cy="{py:.1f}" r="9" fill="none" stroke="#fff" stroke-width="1.5"/>')
    o.append(f'<polygon points="{px+6:.1f},{py:.1f} {px-4:.1f},{py-4:.1f} {px-4:.1f},{py+4:.1f}" fill="{arr}"/>')
    T(px-8,py-5,cs,'#e0e0e0','text-anchor="end" font-weight="bold"'); T(px-8,py+13,lvl,'#e0e0e0','text-anchor="end" font-size="10"')
    if sel: o.append(f'<circle cx="{px:.1f}" cy="{ay:.1f}" r="7" fill="none" stroke="#fff" stroke-width="1.5"/>')
    o.append(f'<circle cx="{px:.1f}" cy="{ay:.1f}" r="3.5" fill="{arr}"/>')
def svg():
    return (f'<svg viewBox="0 0 {W} {H:.0f}" style="display: block; width: 100%; height: auto;" font-family="ui-monospace,Menlo,monospace" font-size="11" role="img" '
            'aria-label="PAR frame of 26L: AFR47KX on the glide path, 6.2 NM out, EZY82QT and DLH9KR behind">' + ''.join(o) + '</svg>')


if __name__ == '__main__':
    print(svg())
