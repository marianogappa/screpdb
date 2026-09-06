// In-memory filesystem shim for Go's wasm_exec.js.
// Implements the Node.js `fs` callbacks Go expects at globalThis.fs.
(function () {
  "use strict";

  const ENOENT = -2;
  const EEXIST = -17;
  const ENOTDIR = -20;
  const EISDIR = -21;
  const EINVAL = -22;

  class InMemoryFS {
    constructor() {
      this._files = new Map();   // path → Uint8Array
      this._dirs = new Set(["/"]);
      this._fds = new Map();
      this._nextFd = 100;
      this._outputBuf = "";
      // Go's wasm_exec.js reads these constants. Values match Linux.
      this.constants = {
        O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_TRUNC: 512,
        O_APPEND: 1024, O_EXCL: 128, O_DIRECTORY: 65536,
      };
    }

    writeSync(fd, buf) {
      const decoder = new TextDecoder();
      this._outputBuf += decoder.decode(buf);
      const nl = this._outputBuf.lastIndexOf("\n");
      if (nl !== -1) {
        const line = this._outputBuf.substring(0, nl);
        this._outputBuf = this._outputBuf.substring(nl + 1);
        if (fd === 2) console.error(line);
        else console.log(line);
      }
      return buf.length;
    }

    _normalizePath(p) {
      if (!p.startsWith("/")) p = "/" + p;
      const parts = p.split("/").filter(Boolean);
      const resolved = [];
      for (const part of parts) {
        if (part === ".") continue;
        if (part === "..") { resolved.pop(); continue; }
        resolved.push(part);
      }
      return "/" + resolved.join("/");
    }

    _parentDir(p) {
      const i = p.lastIndexOf("/");
      return i <= 0 ? "/" : p.substring(0, i);
    }

    _ensureParentDirs(p) {
      const parts = p.split("/").filter(Boolean);
      let cur = "";
      for (let i = 0; i < parts.length - 1; i++) {
        cur += "/" + parts[i];
        this._dirs.add(cur);
      }
    }

    _makeStat(isDir, size) {
      const now = new Date();
      return {
        dev: 0, ino: 0, mode: isDir ? 0o40755 : 0o100644, nlink: 1,
        uid: 0, gid: 0, rdev: 0, size: size,
        blksize: 4096, blocks: Math.ceil(size / 512),
        atimeMs: now.getTime(), mtimeMs: now.getTime(),
        ctimeMs: now.getTime(), birthtimeMs: now.getTime(),
        atime: now, mtime: now, ctime: now, birthtime: now,
        isDirectory() { return isDir; },
        isFile() { return !isDir; },
        isSymbolicLink() { return false; },
        isBlockDevice() { return false; },
        isCharacterDevice() { return false; },
        isFIFO() { return false; },
        isSocket() { return false; },
      };
    }

    open(path, flags, mode, callback) {
      path = this._normalizePath(path);
      const O_WRONLY = 1, O_RDWR = 2, O_CREAT = 64, O_TRUNC = 512, O_APPEND = 1024, O_EXCL = 128;
      const writing = (flags & O_WRONLY) || (flags & O_RDWR);
      const creating = flags & O_CREAT;
      const truncating = flags & O_TRUNC;

      if (this._dirs.has(path)) {
        const fd = this._nextFd++;
        this._fds.set(fd, { path, isDir: true, pos: 0 });
        callback(null, fd);
        return;
      }

      if (!this._files.has(path)) {
        if (!creating) { callback(this._enoent(path)); return; }
        this._ensureParentDirs(path);
        this._files.set(path, new Uint8Array(0));
      } else if (truncating) {
        this._files.set(path, new Uint8Array(0));
      }

      const fd = this._nextFd++;
      this._fds.set(fd, { path, isDir: false, pos: 0 });
      callback(null, fd);
    }

    close(fd, callback) {
      this._fds.delete(fd);
      callback(null);
    }

    read(fd, buffer, offset, length, position, callback) {
      const entry = this._fds.get(fd);
      if (!entry || entry.isDir) { callback(this._enoent("fd:" + fd)); return; }
      const data = this._files.get(entry.path);
      if (!data) { callback(this._enoent(entry.path)); return; }
      const pos = position !== null && position !== undefined ? position : entry.pos;
      const end = Math.min(pos + length, data.length);
      const bytesRead = end - pos;
      if (bytesRead > 0) {
        buffer.set(data.subarray(pos, end), offset);
      }
      entry.pos = end;
      callback(null, bytesRead);
    }

    write(fd, buf, offset, length, position, callback) {
      if (fd === 1 || fd === 2) {
        // stdout / stderr
        const decoder = new TextDecoder();
        const text = decoder.decode(buf.subarray(offset, offset + length));
        if (fd === 1) console.log(text.replace(/\n$/, ""));
        else console.error(text.replace(/\n$/, ""));
        callback(null, length);
        return;
      }
      const entry = this._fds.get(fd);
      if (!entry || entry.isDir) { callback(this._enoent("fd:" + fd)); return; }
      const data = this._files.get(entry.path) || new Uint8Array(0);
      const pos = position !== null && position !== undefined ? position : entry.pos;
      const end = pos + length;
      let newData;
      if (end > data.length) {
        newData = new Uint8Array(end);
        newData.set(data);
      } else {
        newData = new Uint8Array(data);
      }
      newData.set(buf.subarray(offset, offset + length), pos);
      this._files.set(entry.path, newData);
      entry.pos = end;
      callback(null, length);
    }

    stat(path, callback) {
      path = this._normalizePath(path);
      if (this._dirs.has(path)) {
        callback(null, this._makeStat(true, 0));
      } else if (this._files.has(path)) {
        callback(null, this._makeStat(false, this._files.get(path).length));
      } else {
        callback(this._enoent(path));
      }
    }

    lstat(path, callback) { this.stat(path, callback); }

    fstat(fd, callback) {
      const entry = this._fds.get(fd);
      if (!entry) { callback(this._enoent("fd:" + fd)); return; }
      if (entry.isDir) { callback(null, this._makeStat(true, 0)); return; }
      const data = this._files.get(entry.path);
      callback(null, this._makeStat(false, data ? data.length : 0));
    }

    mkdir(path, perm, callback) {
      path = this._normalizePath(path);
      if (this._dirs.has(path) || this._files.has(path)) {
        callback(null);
        return;
      }
      this._ensureParentDirs(path);
      this._dirs.add(path);
      callback(null);
    }

    readdir(path, callback) {
      path = this._normalizePath(path);
      if (!this._dirs.has(path)) { callback(this._enoent(path)); return; }
      const prefix = path === "/" ? "/" : path + "/";
      const entries = new Set();
      for (const p of this._files.keys()) {
        if (p.startsWith(prefix)) {
          const rest = p.substring(prefix.length);
          const name = rest.split("/")[0];
          if (name) entries.add(name);
        }
      }
      for (const d of this._dirs) {
        if (d.startsWith(prefix) && d !== path) {
          const rest = d.substring(prefix.length);
          const name = rest.split("/")[0];
          if (name) entries.add(name);
        }
      }
      callback(null, Array.from(entries));
    }

    unlink(path, callback) {
      path = this._normalizePath(path);
      this._files.delete(path);
      callback(null);
    }

    rename(from, to, callback) {
      from = this._normalizePath(from);
      to = this._normalizePath(to);
      if (this._files.has(from)) {
        this._ensureParentDirs(to);
        this._files.set(to, this._files.get(from));
        this._files.delete(from);
      }
      callback(null);
    }

    rmdir(path, callback) {
      path = this._normalizePath(path);
      this._dirs.delete(path);
      callback(null);
    }

    chmod(path, mode, callback) { callback(null); }
    fchmod(fd, mode, callback) { callback(null); }
    chown(path, uid, gid, callback) { callback(null); }
    fchown(fd, uid, gid, callback) { callback(null); }
    lchown(path, uid, gid, callback) { callback(null); }
    link(path, link, callback) { callback(null); }
    readlink(path, callback) { callback(this._enoent(path)); }
    symlink(path, link, callback) { callback(null); }
    truncate(path, length, callback) { callback(null); }
    utimes(path, atime, mtime, callback) { callback(null); }
    fsync(fd, callback) { callback(null); }
    ftruncate(fd, length, callback) {
      const entry = this._fds.get(fd);
      if (!entry) { callback(null); return; }
      const data = this._files.get(entry.path) || new Uint8Array(0);
      const newData = new Uint8Array(length);
      newData.set(data.subarray(0, Math.min(data.length, length)));
      this._files.set(entry.path, newData);
      callback(null);
    }

    _enoent(path) {
      const err = new Error("ENOENT: no such file or directory, " + path);
      err.code = "ENOENT";
      return err;
    }
  }

  // Always install our full FS. We provide writeSync and constants, which
  // wasm_exec.js needs, plus all the async callbacks Go's os package uses.
  globalThis.fs = new InMemoryFS();

  globalThis.__screpdb_fs = globalThis.fs;
})();
