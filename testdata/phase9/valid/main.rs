trait Speak {
    fn speak(&self);
}

struct Cat {}

impl Speak for Cat {
    fn speak(&self) {}
}

fn call<T: Speak>(x: T) {}

fn main() {
    let c = Cat;
    call(c);
}
