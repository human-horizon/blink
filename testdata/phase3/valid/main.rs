fn swap(a: &mut i32, b: &mut i32) {
    let tmp = *a;
    *a = *b;
    *b = tmp;
}

fn main() -> i32 {
    let mut x: i32 = 1;
    let mut y: i32 = 2;
    swap(&mut x, &mut y);
    x + y
}
