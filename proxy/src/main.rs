use hyper::service::{make_service_fn, service_fn};
use hyper::Server;
use std::convert::Infallible;
use util::Backend;
mod cm;
mod cmthroughput;
mod hotel;
mod hotelwriteheavy;
mod hotelwriteheavyhotkeys;
mod movie;
mod movieprefetch;
mod moviewriteheavy;
mod moviewriteheavyhotkeys;
mod social;
mod socialprefetch;
mod socialwriteheavy;
mod socialwriteheavyhotkeys;
mod boutique;
mod boutiqueprefetch;
mod boutiquewriteheavy;
mod boutiquewriteheavyhotkeys;
mod twoservices;
mod fanin;
mod util;
use clap::Parser;

// Don't forget to export inner structs
// They're used in the macro
use cm::CM;
use cmthroughput::CMThroughput;
use hotel::Hotel;
use hotelwriteheavy::HotelWriteHeavy;
use hotelwriteheavyhotkeys::HotelWriteHeavyHotKeys;
use movie::Movie;
use movieprefetch::MoviePrefetch;
use moviewriteheavy::MovieWriteHeavy;
use moviewriteheavyhotkeys::MovieWriteHeavyHotKeys;
use social::Social;
use socialprefetch::SocialPrefetch;
use socialwriteheavy::SocialWriteHeavy;
use socialwriteheavyhotkeys::SocialWriteHeavyHotKeys;
use boutique::Boutique;
use boutiqueprefetch::BoutiquePrefetch;
use boutiquewriteheavy::BoutiqueWriteHeavy;
use boutiquewriteheavyhotkeys::BoutiqueWriteHeavyHotKeys;
use twoservices::TwoServices;
use fanin::Fanin;

#[derive(Parser)]
pub enum App {
    CM(CM),
    CMThroughput(CMThroughput),
    Hotel(Hotel),
    Social(Social),
    Boutique(Boutique),
    Movie(Movie),
    TwoServices(TwoServices),
    Fanin(Fanin),
    HotelWriteHeavy(HotelWriteHeavy),
    HotelWriteHeavyHotKeys(HotelWriteHeavyHotKeys),
    BoutiquePrefetch(BoutiquePrefetch),
    BoutiqueWriteHeavy(BoutiqueWriteHeavy),
    BoutiqueWriteHeavyHotKeys(BoutiqueWriteHeavyHotKeys),
    MoviePrefetch(MoviePrefetch),
    MovieWriteHeavy(MovieWriteHeavy),
    MovieWriteHeavyHotKeys(MovieWriteHeavyHotKeys),
    SocialPrefetch(SocialPrefetch),
    SocialWriteHeavy(SocialWriteHeavy),
    SocialWriteHeavyHotKeys(SocialWriteHeavyHotKeys),
}

impl_backend!(CM, CMThroughput, Hotel, Social, Boutique, Movie, TwoServices, Fanin, HotelWriteHeavy, HotelWriteHeavyHotKeys, BoutiquePrefetch, BoutiqueWriteHeavy, BoutiqueWriteHeavyHotKeys, MoviePrefetch, MovieWriteHeavy, MovieWriteHeavyHotKeys, SocialPrefetch, SocialWriteHeavy, SocialWriteHeavyHotKeys);

#[tokio::main(worker_threads = 12)]
async fn main() {
    tracing_subscriber::fmt::init();
    let app = App::parse();
    app.prepare().await;
    app.run().await;
}
