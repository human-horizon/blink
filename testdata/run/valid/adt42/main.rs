fn find_flag(v: i32) -> Option<i32> {
    if v > 10 {
        Some(v)
    } else {
        None
    }
}

fn divide(a: i32, b: i32) -> Result<i32, i32> {
    if b == 0 {
        Err(1)
    } else {
        Ok(a)
    }
}

fn main() -> i32 {
    let a = find_flag(15);
    let b = find_flag(5);
    let x = match a {
        Some(n) => n,
        None => 0,
    };
    let y = match b {
        Some(n) => n,
        None => 1,
    };
    let z = a.unwrap();
    let w = if b.is_none() { 5 } else { 0 };
    let r = divide(6, 2);
    let q = match r {
        Ok(v) => v,
        Err(_) => 0,
    };
    x + y + z + w + q
}
