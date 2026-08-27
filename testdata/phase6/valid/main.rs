fn id<'a>(x: &'a i32) -> &'a i32 {
    x
}

fn main() {
    let v: i32 = 1;
    id(&v);
}
