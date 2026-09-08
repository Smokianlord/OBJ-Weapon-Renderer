import struct, sys, os
from pathlib import Path

RT_ICON=3
RT_GROUP_ICON=14
LANG_EN_US=0x0409
SEC_CHARS=0x40000040  # initialized data | read

def align(v,a): return (v+a-1)//a*a

def parse_ico(path):
    b=Path(path).read_bytes()
    if len(b)<6: raise ValueError('ICO too small')
    reserved,typ,count=struct.unpack_from('<HHH',b,0)
    if reserved!=0 or typ!=1 or count<1: raise ValueError('Invalid ICO header')
    entries=[]
    for i in range(count):
        off=6+i*16
        if off+16>len(b): raise ValueError('Truncated ICO directory')
        w,h,cc,res,planes,bpp,size,dataoff=struct.unpack_from('<BBBBHHII',b,off)
        if dataoff+size>len(b): raise ValueError('Truncated ICO image')
        entries.append(dict(w=w,h=h,cc=cc,res=res,planes=planes,bpp=bpp,size=size,data=b[dataoff:dataoff+size]))
    return entries

def build_rsrc(icons, base_rva):
    n=len(icons)
    # Directory layout
    root_off=0
    root_size=16+2*8
    type3_off=align(root_off+root_size,4)
    type3_size=16+n*8
    cur=align(type3_off+type3_size,4)
    icon_lang=[]
    for _ in range(n):
        icon_lang.append(cur); cur=align(cur+24,4)
    type14_off=cur; cur=align(cur+24,4)
    group_lang_off=cur; cur=align(cur+24,4)
    data_entries_off=cur
    icon_data_entry=[]
    for _ in range(n):
        icon_data_entry.append(cur); cur+=16
    group_data_entry=cur; cur+=16
    cur=align(cur,4)

    icon_blob=[]
    for ic in icons:
        icon_blob.append(cur); cur=align(cur+len(ic['data']),4)

    # GRPICONDIR payload
    grp=bytearray(struct.pack('<HHH',0,1,n))
    for idx,ic in enumerate(icons,1):
        grp += struct.pack('<BBBBHHIH', ic['w'], ic['h'], ic['cc'], ic['res'], ic['planes'], ic['bpp'], ic['size'], idx)
    group_blob_off=cur; cur=align(cur+len(grp),4)

    buf=bytearray(cur)
    def dir_header(off,nid):
        struct.pack_into('<IIHHHH',buf,off,0,0,0,0,0,nid)
    def entry(off,idv,target,isdir):
        struct.pack_into('<II',buf,off,idv,(0x80000000 if isdir else 0)|target)

    # root
    dir_header(root_off,2)
    entry(root_off+16,RT_ICON,type3_off,True)
    entry(root_off+24,RT_GROUP_ICON,type14_off,True)

    # RT_ICON type directory
    dir_header(type3_off,n)
    for i,suboff in enumerate(icon_lang,1):
        entry(type3_off+16+(i-1)*8,i,suboff,True)
        dir_header(suboff,1)
        entry(suboff+16,LANG_EN_US,icon_data_entry[i-1],False)

    # RT_GROUP_ICON directory and language
    dir_header(type14_off,1)
    entry(type14_off+16,1,group_lang_off,True)
    dir_header(group_lang_off,1)
    entry(group_lang_off+16,LANG_EN_US,group_data_entry,False)

    # data entries and blobs
    for i,ic in enumerate(icons):
        boff=icon_blob[i]
        struct.pack_into('<IIII',buf,icon_data_entry[i],base_rva+boff,len(ic['data']),0,0)
        buf[boff:boff+len(ic['data'])]=ic['data']
    struct.pack_into('<IIII',buf,group_data_entry,base_rva+group_blob_off,len(grp),0,0)
    buf[group_blob_off:group_blob_off+len(grp)]=grp
    return bytes(buf)

def patch_exe(exe_path, ico_path, out_path):
    data=bytearray(Path(exe_path).read_bytes())
    if data[:2]!=b'MZ': raise ValueError('Not a PE/MZ executable')
    peoff=struct.unpack_from('<I',data,0x3c)[0]
    if data[peoff:peoff+4]!=b'PE\0\0': raise ValueError('Invalid PE signature')
    coff=peoff+4
    machine,nsects=struct.unpack_from('<HH',data,coff)
    if machine!=0x8664: raise ValueError(f'Expected x64 PE, got machine 0x{machine:04x}')
    optsz=struct.unpack_from('<H',data,coff+16)[0]
    opt=coff+20
    magic=struct.unpack_from('<H',data,opt)[0]
    if magic!=0x20b: raise ValueError('Expected PE32+ executable')
    sect_align=struct.unpack_from('<I',data,opt+32)[0]
    file_align=struct.unpack_from('<I',data,opt+36)[0]
    size_headers=struct.unpack_from('<I',data,opt+60)[0]
    sectab=opt+optsz
    newhdr=sectab+nsects*40
    if newhdr+40>size_headers:
        raise ValueError('No space in PE headers for another section')
    # Reject existing .rsrc to avoid duplicate resources
    last_end_rva=0
    for i in range(nsects):
        sh=sectab+i*40
        name=bytes(data[sh:sh+8]).split(b'\0',1)[0]
        vsize,vaddr,rawsize,rawptr=struct.unpack_from('<IIII',data,sh+8)
        if name==b'.rsrc': raise ValueError('Executable already has .rsrc')
        last_end_rva=max(last_end_rva,vaddr+max(vsize,rawsize))
    rsrc_rva=align(last_end_rva,sect_align)
    rsrc=build_rsrc(parse_ico(ico_path),rsrc_rva)
    rawptr=align(len(data),file_align)
    rawsize=align(len(rsrc),file_align)
    if len(data)<rawptr: data += b'\0'*(rawptr-len(data))
    data += rsrc + b'\0'*(rawsize-len(rsrc))

    # section header
    data[newhdr:newhdr+8]=b'.rsrc\0\0\0'
    struct.pack_into('<IIIIIIHHI',data,newhdr+8,len(rsrc),rsrc_rva,rawsize,rawptr,0,0,0,0,SEC_CHARS)
    # COFF section count
    struct.pack_into('<H',data,coff+2,nsects+1)
    # Optional header SizeOfInitializedData
    init_data=struct.unpack_from('<I',data,opt+8)[0]
    struct.pack_into('<I',data,opt+8,init_data+rawsize)
    # SizeOfImage
    struct.pack_into('<I',data,opt+56,align(rsrc_rva+len(rsrc),sect_align))
    # Checksum invalidated -> zero
    struct.pack_into('<I',data,opt+64,0)
    # Resource data directory index 2: PE32+ starts at +112
    nrv=struct.unpack_from('<I',data,opt+108)[0]
    if nrv<3: raise ValueError('PE has no resource data directory slot')
    struct.pack_into('<II',data,opt+112+2*8,rsrc_rva,len(rsrc))

    Path(out_path).write_bytes(data)
    return dict(rsrc_rva=rsrc_rva, rsrc_size=len(rsrc), rawptr=rawptr, rawsize=rawsize, icons=len(parse_ico(ico_path)))

if __name__=='__main__':
    if len(sys.argv)!=4:
        print('usage: embed_pe_icon.py input.exe icon.ico output.exe',file=sys.stderr); sys.exit(2)
    info=patch_exe(sys.argv[1],sys.argv[2],sys.argv[3])
    print(info)
