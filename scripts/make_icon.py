"""Generate a multi-resolution Windows icon without additional dependencies."""
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[1]
POLYGON = [(14,16),(50,16),(50,23),(27,41),(50,41),(50,49),(14,49),(14,41),(37,24),(14,24)]

def inside(x, y):
    hit = False
    for i, (ax, ay) in enumerate(POLYGON):
        bx, by = POLYGON[i-1]
        if (ay > y) != (by > y) and x < (bx-ax)*(y-ay)/(by-ay)+ax:
            hit = not hit
    return hit

def bitmap(size):
    pixels = bytearray()
    for py in reversed(range(size)):
        for px in range(size):
            x, y = (px+.5)*64/size, (py+.5)*64/size
            color = (37,44,41,255)
            if min(x,y,64-x,64-y) < 2:
                color = (174,180,158,255)
            if inside(x,y):
                color = (223,195,131,255)
            if (7<x<25 and 7<y<9) or (7<x<9 and 7<y<25):
                color = (228,232,211,255)
            if (x-53)**2+(y-54)**2 < 9:
                color = (164,202,112,255)
            r,g,b,a = color
            pixels.extend((b,g,r,a))
    mask = bytes(((size+31)//32)*4*size)
    header = struct.pack('<IiiHHIIiiII',40,size,size*2,1,32,0,len(pixels),0,0,0,0)
    return header+pixels+mask

sizes = (16,24,32,48,64,128,256)
images = [bitmap(s) for s in sizes]
offset = 6+16*len(sizes)
entries = bytearray()
for size, data in zip(sizes,images):
    entries.extend(struct.pack('<BBBBHHII',size%256,size%256,0,0,1,32,len(data),offset))
    offset += len(data)
icon = struct.pack('<HHH',0,1,len(sizes))+entries+b''.join(images)
dest = ROOT/'build'/'windows'/'icon.ico'
dest.parent.mkdir(parents=True,exist_ok=True)
dest.write_bytes(icon)
