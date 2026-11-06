// Regenerate the simple app icon on macOS: swift scripts/make-icon.swift
import AppKit
import Foundation
let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
let folder = root.appendingPathComponent("packaging/icons")
let iconset = folder.appendingPathComponent("AppIcon.iconset")
try FileManager.default.createDirectory(at: iconset, withIntermediateDirectories: true)
func render(_ size: Int) -> Data {
 let bitmap = NSBitmapImageRep(bitmapDataPlanes:nil,pixelsWide:size,pixelsHigh:size,bitsPerSample:8,samplesPerPixel:4,hasAlpha:true,isPlanar:false,colorSpaceName:.deviceRGB,bytesPerRow:0,bitsPerPixel:0)!
 NSGraphicsContext.saveGraphicsState(); NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep:bitmap)
 let transform = NSAffineTransform(); transform.scale(by:CGFloat(size)/1024); transform.concat()
 NSColor(calibratedRed:0.0,green:0.48,blue:0.46,alpha:1).setFill()
 NSBezierPath(roundedRect:NSRect(x:64,y:64,width:896,height:896),xRadius:192,yRadius:192).fill()
 NSColor.white.setFill()
 NSBezierPath(rect:NSRect(x:448,y:550,width:128,height:240)).fill()
 let arrow = NSBezierPath(); arrow.move(to:NSPoint(x:448,y:790)); arrow.line(to:NSPoint(x:576,y:790)); arrow.line(to:NSPoint(x:576,y:554)); arrow.line(to:NSPoint(x:704,y:554)); arrow.line(to:NSPoint(x:512,y:362)); arrow.line(to:NSPoint(x:320,y:554)); arrow.line(to:NSPoint(x:448,y:554)); arrow.close(); arrow.fill()
 NSColor.white.setStroke()
 let board = NSBezierPath(roundedRect:NSRect(x:264,y:222,width:496,height:128),xRadius:36,yRadius:36); board.lineWidth=32; board.stroke()
 for x in [352,448,544] {NSBezierPath(ovalIn:NSRect(x:x,y:270,width:32,height:32)).fill()}
 NSGraphicsContext.restoreGraphicsState()
 return bitmap.representation(using:.png,properties:[:])!
}
for size in [16,32,128,256,512] {
 try render(size).write(to:iconset.appendingPathComponent("icon_\(size)x\(size).png"))
 try render(size*2).write(to:iconset.appendingPathComponent("icon_\(size)x\(size)@2x.png"))
}
try render(512).write(to:folder.appendingPathComponent("app-icon.png"))
func be(_ value: UInt32) -> Data {var v=value.bigEndian; return withUnsafeBytes(of:&v){Data($0)}}
func le16(_ value: UInt16) -> Data {var v=value.littleEndian; return withUnsafeBytes(of:&v){Data($0)}}
func le32(_ value: UInt32) -> Data {var v=value.littleEndian; return withUnsafeBytes(of:&v){Data($0)}}
var chunks=Data()
for (tag,size) in [("icp4",16),("icp5",32),("icp6",64),("ic07",128),("ic08",256),("ic09",512),("ic10",1024)] {
 let png=render(size);chunks.append(Data(tag.utf8));chunks.append(be(UInt32(png.count+8)));chunks.append(png)
}
var icns=Data("icns".utf8);icns.append(be(UInt32(chunks.count+8)));icns.append(chunks)
try icns.write(to:folder.appendingPathComponent("AppIcon.icns"))
let sizes=[16,32,128,256]
var ico=le16(0);ico.append(le16(1));ico.append(le16(UInt16(sizes.count)))
var payload=Data(), offset=UInt32(6+16*sizes.count)
for size in sizes {
 let png=render(size);ico.append(contentsOf:[UInt8(size%256),UInt8(size%256),0,0]);ico.append(le16(1));ico.append(le16(32));ico.append(le32(UInt32(png.count)));ico.append(le32(offset));payload.append(png);offset+=UInt32(png.count)
}
ico.append(payload);try ico.write(to:folder.appendingPathComponent("AppIcon.ico"))
