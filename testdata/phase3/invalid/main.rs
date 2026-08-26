fn main() {
    let mut x: i32 = 1;
    let a = &mut x;
    let b = &x;
    *a = *b;
}
